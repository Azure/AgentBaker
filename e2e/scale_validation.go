package e2e

import (
	"context"
	"errors"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	scaleValidationImage   = "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.2"
	scaleValidationTimeout = 10 * time.Minute
)

// ValidateNodeCanScaleToCapacity fills every currently available pod slot on the scenario node.
func ValidateNodeCanScaleToCapacity(ctx context.Context, s *Scenario) (retErr error) {
	nodeName := s.Runtime.VM.KubeName
	node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get scenario node %q: %w", nodeName, err)
	}

	pods, err := s.Runtime.Kube.Typed.CoreV1().Pods(corev1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName).String(),
	})
	if err != nil {
		return fmt.Errorf("list pods on scenario node %q: %w", nodeName, err)
	}

	targetReplicas, err := availablePodSlots(node, pods.Items)
	if err != nil {
		return err
	}

	deployment := scaleValidationDeployment(s, targetReplicas)
	if err := setScenarioNodeOwnerReference(ctx, s, deployment); err != nil {
		return err
	}

	s.Logger.Logf("scaling deployment %q to fill %d available pod slots on node %q", deployment.Name, targetReplicas, nodeName)
	if err := s.Runtime.Kube.CreateDeployment(ctx, deployment); err != nil {
		return fmt.Errorf("create scale validation deployment %q: %w", deployment.Name, err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
		if err := cleanupScaleValidationDeployment(cleanupCtx, s, deployment); err != nil {
			retErr = errors.Join(retErr, err)
		}
	}()

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, scaleValidationTimeout, true, func(ctx context.Context) (bool, error) {
		node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("get node %q while scaling pods: %w", nodeName, err)
		}
		if !isNodeReady(node) {
			return false, fmt.Errorf("node %q became NotReady while scaling deployment %q to %d pods", nodeName, deployment.Name, targetReplicas)
		}

		current, err := s.Runtime.Kube.Typed.AppsV1().Deployments(deployment.Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("get scale validation deployment %q: %w", deployment.Name, err)
		}
		s.Logger.Logf("scale validation deployment %q: %d/%d replicas ready", deployment.Name, current.Status.ReadyReplicas, targetReplicas)
		return current.Status.ReadyReplicas == targetReplicas, nil
	})
	if err != nil {
		return fmt.Errorf("scale deployment %q to node %q capacity (%d replicas): %w", deployment.Name, nodeName, targetReplicas, err)
	}

	s.Logger.Logf("node %q successfully ran %d scale validation pods", nodeName, targetReplicas)
	return nil
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
	if err := deployments.Delete(ctx, deployment.Name, metav1.DeleteOptions{PropagationPolicy: &propagation}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete scale validation deployment %q: %w", deployment.Name, err)
	}

	selector := "app=" + deployment.Spec.Template.Labels["app"]
	err := wait.PollUntilContextTimeout(ctx, time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		pods, err := s.Runtime.Kube.Typed.CoreV1().Pods(deployment.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			return false, err
		}
		return len(pods.Items) == 0, nil
	})
	if err != nil {
		return fmt.Errorf("wait for pods matching %q from deployment %q to be deleted: %w", selector, deployment.Name, err)
	}
	return nil
}
