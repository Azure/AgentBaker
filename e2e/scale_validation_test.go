package e2e

import "testing"

func TestScaleValidationDeploymentTargetsOnlyScenarioNode(t *testing.T) {
	t.Parallel()

	s := &Scenario{Runtime: &ScenarioRuntime{VM: &ScenarioVM{KubeName: "scenario-node"}}}
	deployment := scaleValidationDeployment(s, 42)

	if got, want := *deployment.Spec.Replicas, int32(42); got != want {
		t.Fatalf("replicas = %d, want %d", got, want)
	}
	if got, want := deployment.Spec.Template.Spec.NodeSelector["kubernetes.io/hostname"], "scenario-node"; got != want {
		t.Fatalf("node selector hostname = %q, want %q", got, want)
	}
	if got, want := deployment.Spec.Template.Spec.Containers[0].Image, scaleValidationImage; got != want {
		t.Fatalf("image = %q, want %q", got, want)
	}
	if deployment.Spec.Template.Spec.HostNetwork {
		t.Fatal("scale validation pods must use the normal pod network")
	}
	if got, want := deployment.Spec.Selector.MatchLabels["app"], deployment.Spec.Template.Labels["app"]; got != want {
		t.Fatalf("selector label = %q, pod template label = %q", got, want)
	}
}
