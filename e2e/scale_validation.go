package e2e

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	scaleValidationImage          = "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.2"
	scaleValidationTimeout        = 10 * time.Minute
	scaleValidationCleanupTimeout = 5 * time.Minute
	scaleValidationNotReadyLimit  = 3
	scaleValidationStableLimit    = 13
)

// ValidateNodeCanScaleToCapacity fills the scenario node's allocatable pod capacity.
func ValidateNodeCanScaleToCapacity(ctx context.Context, s *Scenario) (retErr error) {
	nodeName := s.Runtime.VM.KubeName
	node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get scenario node %q: %w", nodeName, err)
	}

	podCapacity, err := allocatablePodCapacity(node)
	if err != nil {
		return err
	}

	pods, err := s.Runtime.Kube.Typed.CoreV1().Pods(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName).String(),
	})
	if err != nil {
		return fmt.Errorf("list pods on scenario node %q: %w", nodeName, err)
	}

	initialReplicas, err := availableScaleReplicas(podCapacity, nodeName, "", pods.Items)
	if err != nil {
		return err
	}

	deployment := scaleValidationDeployment(s, initialReplicas)
	if err := setScenarioNodeOwnerReference(ctx, s, deployment); err != nil {
		return err
	}

	s.Logger.Logf("scaling deployment %q to %d available slots on node %q", deployment.Name, initialReplicas, nodeName)
	if _, err := s.Runtime.Kube.Typed.AppsV1().Deployments(deployment.Namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create scale validation deployment %q: %w", deployment.Name, err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scaleValidationCleanupTimeout)
		defer cancel()
		if err := cleanupScaleValidationDeployment(cleanupCtx, s, deployment); err != nil {
			retErr = errors.Join(retErr, err)
		}
	}()

	var lastPollErr error
	finalReplicas := initialReplicas
	consecutiveNotReady := 0
	consecutiveStable := 0
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, scaleValidationTimeout, true, func(ctx context.Context) (bool, error) {
		node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, fmt.Errorf("scenario node %q disappeared while scaling: %w", nodeName, err)
			}
			consecutiveStable = 0
			lastPollErr = fmt.Errorf("get node %q while scaling pods: %w", nodeName, err)
			s.Logger.Log(lastPollErr.Error())
			return false, nil
		}
		if !scaleValidationNodeReady(node) {
			consecutiveNotReady++
			consecutiveStable = 0
			if consecutiveNotReady >= scaleValidationNotReadyLimit {
				return false, fmt.Errorf(
					"node %q remained NotReady for %d consecutive checks while scaling deployment %q to %d pods",
					nodeName,
					consecutiveNotReady,
					deployment.Name,
					finalReplicas,
				)
			}
			return false, nil
		}
		consecutiveNotReady = 0

		pods, err := s.Runtime.Kube.Typed.CoreV1().Pods(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName).String(),
		})
		if err != nil {
			consecutiveStable = 0
			lastPollErr = fmt.Errorf("list pods on node %q while scaling: %w", nodeName, err)
			s.Logger.Log(lastPollErr.Error())
			return false, nil
		}
		lastPollErr = nil

		desiredReplicas, err := availableScaleReplicas(
			podCapacity,
			nodeName,
			deployment.Spec.Template.Labels["app"],
			pods.Items,
		)
		if err != nil {
			return false, err
		}

		current, err := s.Runtime.Kube.Typed.AppsV1().Deployments(deployment.Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
		if err != nil {
			consecutiveStable = 0
			lastPollErr = fmt.Errorf("get scale validation deployment %q: %w", deployment.Name, err)
			s.Logger.Log(lastPollErr.Error())
			return false, nil
		}

		if current.Spec.Replicas == nil || *current.Spec.Replicas != desiredReplicas {
			current.Spec.Replicas = &desiredReplicas
			if _, err := s.Runtime.Kube.Typed.AppsV1().Deployments(deployment.Namespace).Update(ctx, current, metav1.UpdateOptions{}); err != nil {
				consecutiveStable = 0
				lastPollErr = fmt.Errorf("update scale validation deployment %q to %d replicas: %w", deployment.Name, desiredReplicas, err)
				s.Logger.Log(lastPollErr.Error())
				return false, nil
			}
			finalReplicas = desiredReplicas
			consecutiveStable = 0
			s.Logger.Logf("adjusted scale validation deployment %q to %d available slots", deployment.Name, desiredReplicas)
			return false, nil
		}

		finalReplicas = desiredReplicas
		lastPollErr = nil
		fullyReady := current.Status.ObservedGeneration >= current.Generation &&
			current.Status.UpdatedReplicas == desiredReplicas &&
			current.Status.ReadyReplicas == desiredReplicas
		if fullyReady {
			consecutiveStable++
		} else {
			consecutiveStable = 0
		}
		s.Logger.Logf(
			"scale validation deployment %q: %d/%d replicas ready, stability %d/%d",
			deployment.Name,
			current.Status.ReadyReplicas,
			desiredReplicas,
			consecutiveStable,
			scaleValidationStableLimit,
		)
		return consecutiveStable >= scaleValidationStableLimit, nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf(
				"test context ended before deployment %q filled node %q capacity (%d pods): %w",
				deployment.Name,
				nodeName,
				finalReplicas,
				errors.Join(err, lastPollErr),
			)
		}
		return fmt.Errorf(
			"scale deployment %q to node %q capacity (%d pods): %w",
			deployment.Name,
			nodeName,
			finalReplicas,
			errors.Join(err, lastPollErr),
		)
	}

	s.Logger.Logf("deployment %q successfully ran all %d requested replicas on node %q", deployment.Name, finalReplicas, nodeName)
	return nil
}

func allocatablePodCapacity(node *corev1.Node) (int32, error) {
	capacity, found := node.Status.Allocatable[corev1.ResourcePods]
	if !found {
		return 0, fmt.Errorf("node %q does not advertise allocatable pods", node.Name)
	}

	value := capacity.Value()
	if value <= 0 || value > math.MaxInt32 {
		return 0, fmt.Errorf("node %q advertises invalid allocatable pod capacity %d", node.Name, value)
	}
	return int32(value), nil
}

func scaleValidationNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func availableScaleReplicas(capacity int32, nodeName, scaleAppLabel string, pods []corev1.Pod) (int32, error) {
	var unavailableSlots int32
	for i := range pods {
		pod := &pods[i]
		if pod.Spec.NodeName != nodeName {
			continue
		}

		if scaleAppLabel != "" && pod.Labels["app"] == scaleAppLabel {
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				return 0, fmt.Errorf("scale validation pod %q unexpectedly entered terminal phase %q", pod.Name, pod.Status.Phase)
			}
			continue
		}
		unavailableSlots++
	}

	available := capacity - unavailableSlots
	if available <= 0 {
		return 0, fmt.Errorf(
			"node %q has no pod slots available for scale validation: capacity=%d unavailable-slots=%d",
			nodeName,
			capacity,
			unavailableSlots,
		)
	}
	return available, nil
}

// scaleValidationDeployment builds a pause-container Deployment pinned to the scenario node.
func scaleValidationDeployment(s *Scenario, replicas int32) *appsv1.Deployment {
	name := uniqueKubernetesResourceName("scale-" + s.Runtime.VM.KubeName)
	labels := map[string]string{"app": name}
	zero := int64(0)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "pause",
						Image:           scaleValidationImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
					}},
					NodeSelector:                  getNodeSelectorForScenario(s),
					Tolerations:                   getPodTolerations(),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: &zero,
				},
			},
		},
	}
}

// cleanupScaleValidationDeployment deletes the Deployment and waits for all of its pods to disappear.
func cleanupScaleValidationDeployment(ctx context.Context, s *Scenario, deployment *appsv1.Deployment) error {
	deployments := s.Runtime.Kube.Typed.AppsV1().Deployments(deployment.Namespace)
	propagation := metav1.DeletePropagationBackground
	var lastDeleteErr error
	err := wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		err := deployments.Delete(ctx, deployment.Name, metav1.DeleteOptions{PropagationPolicy: &propagation})
		if err == nil || apierrors.IsNotFound(err) {
			return true, nil
		}
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsInvalid(err) {
			return false, err
		}

		lastDeleteErr = err
		s.Logger.Logf("error deleting scale validation deployment %q: %v", deployment.Name, err)
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("delete scale validation deployment %q: %w", deployment.Name, errors.Join(err, lastDeleteErr))
	}

	selector := "app=" + deployment.Spec.Template.Labels["app"]
	var lastPollErr error
	err = wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		pods, err := s.Runtime.Kube.Typed.CoreV1().Pods(deployment.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			lastPollErr = err
			s.Logger.Logf("error listing scale validation pods during cleanup: %v", err)
			return false, nil
		}
		lastPollErr = nil
		return len(pods.Items) == 0, nil
	})
	if err != nil {
		return fmt.Errorf(
			"wait for pods matching %q from deployment %q to be deleted: %w",
			selector,
			deployment.Name,
			errors.Join(err, lastPollErr),
		)
	}
	return nil
}
