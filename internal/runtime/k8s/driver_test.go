package k8s

import (
	"context"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/MackDing/hermes-manager/internal/runtime"
	"github.com/MackDing/hermes-manager/internal/storage"
)

func newTestDriver(t *testing.T, opts ...Option) (*Driver, *fake.Clientset) {
	t.Helper()
	fc := fake.NewSimpleClientset()
	allOpts := append([]Option{
		WithClient(fc),
		WithNamespace("test-ns"),
		WithPort("9090"),
		WithImage("test-image:latest"),
	}, opts...)
	d, err := New(allOpts...)
	if err != nil {
		t.Fatalf("unexpected error creating driver: %v", err)
	}
	return d, fc
}

func newTestTask() storage.Task {
	return storage.Task{
		ID:        "task-abc-123",
		SkillName: "hello-skill",
		Parameters: map[string]any{
			"name": "world",
		},
		PolicyContext: storage.PolicyContext{
			User:         "admin",
			Team:         "default",
			ModelAllowed: []string{"demo-model"},
		},
		Runtime:      "k8s",
		State:        storage.TaskStatePending,
		DeadlineSecs: 300,
	}
}

func newTestSkill() storage.Skill {
	return storage.Skill{
		Name:        "hello-skill",
		Version:     "0.1.0",
		Description: "Demo skill that greets a user.",
		Entrypoint:  "run.py",
	}
}

// TestInterfaceCompliance verifies that Driver satisfies runtime.Runtime.
func TestInterfaceCompliance(t *testing.T) {
	var _ runtime.Runtime = (*Driver)(nil)
}

func TestName(t *testing.T) {
	d, _ := newTestDriver(t)
	if got := d.Name(); got != "k8s" {
		t.Errorf("Name() = %q, want %q", got, "k8s")
	}
}

func TestCallbackURL(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		port      string
		want      string
	}{
		{
			name:      "custom namespace and port",
			namespace: "prod",
			port:      "9090",
			want:      "http://hermesmanager.prod.svc.cluster.local:9090/v1/events",
		},
		{
			name:      "default values",
			namespace: "default",
			port:      "8080",
			want:      "http://hermesmanager.default.svc.cluster.local:8080/v1/events",
		},
		{
			name:      "custom namespace with default port",
			namespace: "hermes",
			port:      "8080",
			want:      "http://hermesmanager.hermes.svc.cluster.local:8080/v1/events",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, _ := newTestDriver(t,
				WithNamespace(tt.namespace),
				WithPort(tt.port),
			)
			got := d.CallbackURL()
			if got != tt.want {
				t.Errorf("CallbackURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDispatchCreatesJobAndSecret(t *testing.T) {
	d, fc := newTestDriver(t)
	ctx := context.Background()
	task := newTestTask()
	skill := newTestSkill()

	handle, err := d.Dispatch(ctx, task, skill)
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	expectedJobName := "hermesmanager-" + task.ID

	// Verify handle.
	if handle.RuntimeName != "k8s" {
		t.Errorf("handle.RuntimeName = %q, want %q", handle.RuntimeName, "k8s")
	}
	if handle.ExternalID != expectedJobName {
		t.Errorf("handle.ExternalID = %q, want %q", handle.ExternalID, expectedJobName)
	}

	// Verify Job was created.
	job, err := fc.BatchV1().Jobs("test-ns").Get(ctx, expectedJobName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Job not found: %v", err)
	}

	// Verify Job labels.
	if job.Labels[labelManaged] != "true" {
		t.Errorf("Job label %q = %q, want %q", labelManaged, job.Labels[labelManaged], "true")
	}
	if job.Labels[labelTaskID] != task.ID {
		t.Errorf("Job label %q = %q, want %q", labelTaskID, job.Labels[labelTaskID], task.ID)
	}

	// Verify container env vars.
	containers := job.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	container := containers[0]

	envMap := make(map[string]corev1.EnvVar)
	for _, e := range container.Env {
		envMap[e.Name] = e
	}

	if ev, ok := envMap["HERMESMANAGER_TASK_ID"]; !ok || ev.Value != task.ID {
		t.Errorf("missing or wrong HERMESMANAGER_TASK_ID env var")
	}
	if ev, ok := envMap["HERMESMANAGER_CALLBACK_URL"]; !ok || ev.Value != d.CallbackURL() {
		t.Errorf("missing or wrong HERMESMANAGER_CALLBACK_URL env var")
	}
	if ev, ok := envMap["HERMESMANAGER_AGENT_TOKEN"]; !ok || ev.ValueFrom == nil || ev.ValueFrom.SecretKeyRef == nil {
		t.Errorf("HERMESMANAGER_AGENT_TOKEN should reference a secret")
	} else if ev.ValueFrom.SecretKeyRef.Name != expectedJobName+"-token" {
		t.Errorf("HERMESMANAGER_AGENT_TOKEN secret ref = %q, want %q",
			ev.ValueFrom.SecretKeyRef.Name, expectedJobName+"-token")
	}

	// Verify volume mounts.
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volume mount, got %d", len(container.VolumeMounts))
	}
	vm := container.VolumeMounts[0]
	if vm.MountPath != "/etc/hermesmanager" {
		t.Errorf("volume mount path = %q, want %q", vm.MountPath, "/etc/hermesmanager")
	}
	if !vm.ReadOnly {
		t.Error("volume mount should be read-only")
	}

	// Verify Secret was created with ownerReferences.
	secret, err := fc.CoreV1().Secrets("test-ns").Get(ctx, expectedJobName+"-token", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Secret not found: %v", err)
	}
	if len(secret.OwnerReferences) != 1 {
		t.Fatalf("expected 1 ownerReference, got %d", len(secret.OwnerReferences))
	}
	ownerRef := secret.OwnerReferences[0]
	if ownerRef.Kind != "Job" {
		t.Errorf("ownerRef Kind = %q, want %q", ownerRef.Kind, "Job")
	}
	if ownerRef.Name != expectedJobName {
		t.Errorf("ownerRef Name = %q, want %q", ownerRef.Name, expectedJobName)
	}
	if ownerRef.UID != job.UID {
		t.Errorf("ownerRef UID = %q, want %q", ownerRef.UID, job.UID)
	}

	// Verify Secret has token data.
	if _, ok := secret.Data["token"]; !ok {
		// fake client converts StringData to Data
		if _, ok := secret.StringData["token"]; !ok {
			t.Error("Secret missing 'token' key in Data/StringData")
		}
	}

	// Verify ConfigMap was created.
	cm, err := fc.CoreV1().ConfigMaps("test-ns").Get(ctx, expectedJobName+"-config", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("ConfigMap not found: %v", err)
	}
	if _, ok := cm.Data["task.json"]; !ok {
		t.Error("ConfigMap missing 'task.json' key")
	}
}

func TestDispatchSetsLabels(t *testing.T) {
	d, fc := newTestDriver(t)
	ctx := context.Background()
	task := newTestTask()
	skill := newTestSkill()

	_, err := d.Dispatch(ctx, task, skill)
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	expectedJobName := "hermesmanager-" + task.ID

	// Check Job labels.
	job, err := fc.BatchV1().Jobs("test-ns").Get(ctx, expectedJobName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Job not found: %v", err)
	}
	if job.Labels[labelManaged] != "true" {
		t.Errorf("Job missing %s=true label", labelManaged)
	}

	// Check pod template labels.
	podLabels := job.Spec.Template.Labels
	if podLabels[labelManaged] != "true" {
		t.Errorf("Pod template missing %s=true label", labelManaged)
	}

	// Check Secret labels.
	secret, err := fc.CoreV1().Secrets("test-ns").Get(ctx, expectedJobName+"-token", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Secret not found: %v", err)
	}
	if secret.Labels[labelManaged] != "true" {
		t.Errorf("Secret missing %s=true label", labelManaged)
	}

	// Check ConfigMap labels.
	cm, err := fc.CoreV1().ConfigMaps("test-ns").Get(ctx, expectedJobName+"-config", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("ConfigMap not found: %v", err)
	}
	if cm.Labels[labelManaged] != "true" {
		t.Errorf("ConfigMap missing %s=true label", labelManaged)
	}
}

func TestStatusCountsActiveJobs(t *testing.T) {
	d, fc := newTestDriver(t)
	ctx := context.Background()

	// Create some Jobs with the managed label, varying active status.
	jobs := []struct {
		name   string
		active int32
	}{
		{"job-1", 1},
		{"job-2", 0}, // completed
		{"job-3", 1},
		{"job-4", 0}, // completed
	}

	for _, j := range jobs {
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      j.name,
				Namespace: "test-ns",
				Labels: map[string]string{
					labelManaged: "true",
				},
			},
			Status: batchv1.JobStatus{
				Active: j.active,
			},
		}
		if _, err := fc.BatchV1().Jobs("test-ns").Create(ctx, job, metav1.CreateOptions{}); err != nil {
			t.Fatalf("create test job %s: %v", j.name, err)
		}
	}

	active, max, err := d.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if active != 2 {
		t.Errorf("Status() active = %d, want 2", active)
	}
	if max != maxConcurrency {
		t.Errorf("Status() max = %d, want %d", max, maxConcurrency)
	}
}

func TestStatusEmptyNamespace(t *testing.T) {
	d, _ := newTestDriver(t)
	ctx := context.Background()

	active, max, err := d.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if active != 0 {
		t.Errorf("Status() active = %d, want 0", active)
	}
	if max != maxConcurrency {
		t.Errorf("Status() max = %d, want %d", max, maxConcurrency)
	}
}
