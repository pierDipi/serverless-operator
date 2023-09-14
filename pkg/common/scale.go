package common

import (
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"knative.dev/operator/pkg/apis/operator/base"
)

type component interface {
	SetWorkloads([]base.WorkloadOverride)
	GetWorkloadOverrides() []base.WorkloadOverride
}

// ScaleComponentTo scales the given component to replicas if it's not otherwise specified by component
func ScaleComponentTo(resource component, component types.NamespacedName, replicas int32) {

	found := false
	existing := resource.GetWorkloadOverrides()
	workloads := make([]base.WorkloadOverride, 0, len(existing))
	for _, w := range existing {

		if w.Name == component.Name {
			found = true

			if w.Replicas == nil {
				w = *w.DeepCopy()
				w.Replicas = &replicas
			}
		}

		workloads = append(workloads, w)
	}

	if !found {
		workloads = append(workloads, base.WorkloadOverride{
			Name:     component.Name,
			Replicas: pointer.Int32(replicas),
		})
	}

	resource.SetWorkloads(workloads)
}
