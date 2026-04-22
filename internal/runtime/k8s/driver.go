// Package k8s implements the Kubernetes Job runtime driver for HermesManager.
//
// The k8s driver creates a K8s Job per task, with a per-task Secret for the
// agent bearer token and a ConfigMap volume mount for task.json.
package k8s

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/MackDing/hermes-manager/internal/runtime"
	"github.com/MackDing/hermes-manager/internal/storage"
)

const (
	defaultPort      = "8080"
	defaultNamespace = "default"
	defaultImage     = "hermesmanager/demo-agent:latest"
	envPort          = "HERMESMANAGER_PORT"
	envNamespace     = "HERMESMANAGER_NAMESPACE"
	envImage         = "HERMESMANAGER_AGENT_IMAGE"
	maxConcurrency   = 50

	labelManaged = "hermesmanager.io/managed"
	labelTaskID  = "hermesmanager.io/task-id"
)

func init() {
	runtime.Register("k8s", func() (runtime.Runtime, error) {
		return New()
	})
}

// Driver is the Kubernetes Job execution backend.
type Driver struct {
	client    kubernetes.Interface
	namespace string
	port      string
	image     string
}

// Option configures the K8s driver.
type Option func(*Driver)

// WithClient sets the Kubernetes client (useful for testing with fake clients).
func WithClient(c kubernetes.Interface) Option {
	return func(d *Driver) {
		d.client = c
	}
}

// WithNamespace overrides the target namespace.
func WithNamespace(ns string) Option {
	return func(d *Driver) {
		d.namespace = ns
	}
}

// WithPort overrides the callback port.
func WithPort(port string) Option {
	return func(d *Driver) {
		d.port = port
	}
}

// WithImage overrides the agent container image.
func WithImage(image string) Option {
	return func(d *Driver) {
		d.image = image
	}
}

// New creates a K8s Driver. Without options, it reads configuration from
// environment variables and attempts in-cluster Kubernetes authentication.
func New(opts ...Option) (*Driver, error) {
	d := &Driver{
		namespace: envOrDefault(envNamespace, defaultNamespace),
		port:      envOrDefault(envPort, defaultPort),
		image:     envOrDefault(envImage, defaultImage),
	}

	for _, o := range opts {
		o(d)
	}

	if d.client == nil {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("k8s: in-cluster config: %w", err)
		}
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("k8s: create client: %w", err)
		}
		d.client = client
	}

	return d, nil
}

// Name returns the runtime identifier.
func (d *Driver) Name() string { return "k8s" }

// CallbackURL returns the control-plane events endpoint reachable from
// inside a K8s pod via cluster DNS.
func (d *Driver) CallbackURL() string {
	return "http://hermesmanager." + d.namespace + ".svc.cluster.local:" + d.port + "/v1/events"
}

// taskJSON is the on-disk representation of a task passed to the agent.
type taskJSON struct {
	TaskID          string               `json:"task_id"`
	Skill           string               `json:"skill"`
	Parameters      map[string]any       `json:"parameters"`
	PolicyContext   storage.PolicyContext `json:"policy_context"`
	DeadlineSeconds int                  `json:"deadline_seconds"`
}

// Dispatch creates a K8s Job, ConfigMap, and Secret for the given task.
//
// Resources created:
//  1. ConfigMap with task.json mounted at /etc/hermesmanager/
//  2. Secret containing the agent bearer token
//  3. Job referencing both, with ownerReferences from Secret to Job
func (d *Driver) Dispatch(ctx context.Context, task storage.Task, skill storage.Skill) (runtime.Handle, error) {
	jobName := "hermesmanager-" + task.ID

	// Generate a per-task agent token.
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return runtime.Handle{}, fmt.Errorf("k8s: generate agent token: %w", err)
	}
	agentToken := hex.EncodeToString(tokenBytes)

	// Marshal task.json content.
	tj := taskJSON{
		TaskID:          task.ID,
		Skill:           task.SkillName,
		Parameters:      task.Parameters,
		PolicyContext:    task.PolicyContext,
		DeadlineSeconds: task.DeadlineSecs,
	}
	taskData, err := json.Marshal(tj)
	if err != nil {
		return runtime.Handle{}, fmt.Errorf("k8s: marshal task json: %w", err)
	}

	// 1. Create ConfigMap with task.json.
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName + "-config",
			Namespace: d.namespace,
			Labels: map[string]string{
				labelManaged: "true",
				labelTaskID:  task.ID,
			},
		},
		Data: map[string]string{
			"task.json": string(taskData),
		},
	}
	createdCM, err := d.client.CoreV1().ConfigMaps(d.namespace).Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return runtime.Handle{}, fmt.Errorf("k8s: create configmap: %w", err)
	}

	// 2. Create the Job.
	backoffLimit := int32(0)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: d.namespace,
			Labels: map[string]string{
				labelManaged: "true",
				labelTaskID:  task.ID,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelManaged: "true",
						labelTaskID:  task.ID,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "agent",
							Image: d.image,
							Env: []corev1.EnvVar{
								{Name: "HERMESMANAGER_TASK_ID", Value: task.ID},
								{Name: "HERMESMANAGER_CALLBACK_URL", Value: d.CallbackURL()},
								{
									Name: "HERMESMANAGER_AGENT_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: jobName + "-token",
											},
											Key: "token",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "task-config",
									MountPath: "/etc/hermesmanager",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "task-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: createdCM.Name,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	createdJob, err := d.client.BatchV1().Jobs(d.namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		// Best-effort cleanup of ConfigMap.
		_ = d.client.CoreV1().ConfigMaps(d.namespace).Delete(ctx, cm.Name, metav1.DeleteOptions{})
		return runtime.Handle{}, fmt.Errorf("k8s: create job: %w", err)
	}

	// 3. Create Secret with ownerReferences pointing at the Job.
	isController := true
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName + "-token",
			Namespace: d.namespace,
			Labels: map[string]string{
				labelManaged: "true",
				labelTaskID:  task.ID,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "batch/v1",
					Kind:       "Job",
					Name:       createdJob.Name,
					UID:        createdJob.UID,
					Controller: &isController,
				},
			},
		},
		StringData: map[string]string{
			"token": agentToken,
		},
	}

	if _, err := d.client.CoreV1().Secrets(d.namespace).Create(ctx, secret, metav1.CreateOptions{}); err != nil {
		// Best-effort cleanup.
		_ = d.client.BatchV1().Jobs(d.namespace).Delete(ctx, createdJob.Name, metav1.DeleteOptions{})
		_ = d.client.CoreV1().ConfigMaps(d.namespace).Delete(ctx, cm.Name, metav1.DeleteOptions{})
		return runtime.Handle{}, fmt.Errorf("k8s: create secret: %w", err)
	}

	return runtime.Handle{
		RuntimeName: "k8s",
		ExternalID:  jobName,
	}, nil
}

// Status returns the current load metric for scheduling decisions.
// It counts active Jobs with the hermesmanager.io/managed=true label.
func (d *Driver) Status(ctx context.Context) (int, int, error) {
	selector := labels.Set{labelManaged: "true"}.AsSelector().String()

	jobList, err := d.client.BatchV1().Jobs(d.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("k8s: list jobs: %w", err)
	}

	active := 0
	for _, j := range jobList.Items {
		if j.Status.Active > 0 {
			active++
		}
	}

	return active, maxConcurrency, nil
}

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
