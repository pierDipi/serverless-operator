package eventing

import (
	"context"
	"os"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/operator/pkg/apis/operator/base"
	operatorv1beta1 "knative.dev/operator/pkg/apis/operator/v1beta1"
	"knative.dev/pkg/logging"

	"github.com/openshift-knative/serverless-operator/pkg/common"
)

type scaleComponent struct {
	ke *operatorv1beta1.KnativeEventing
}

func (s *scaleComponent) SetWorkloads(overrides []base.WorkloadOverride) {
	s.ke.Spec.Workloads = overrides
}

func (s *scaleComponent) GetWorkloadOverrides() []base.WorkloadOverride {
	return s.ke.GetSpec().GetWorkloadOverrides()
}

func (e *extension) AutoscaleComponents(ctx context.Context, ke *operatorv1beta1.KnativeEventing) error {

	logger := logging.FromContext(ctx).With(zap.String("component", "autoscaler"))

	sc := &scaleComponent{ke: ke}

	eventingNamespace := os.Getenv(requiredNsEnvName)

	// Core
	eventingController := types.NamespacedName{Namespace: eventingNamespace, Name: "eventing-controller"}
	pingSourceAdapter := types.NamespacedName{Namespace: eventingNamespace, Name: "pingsource-mt-adapter"}
	eventingWebhook := types.NamespacedName{Namespace: eventingNamespace, Name: "eventing-webhook"}
	common.ScaleComponentTo(sc, eventingWebhook, defaultReplicas)
	scaleUpCore := func() {
		common.ScaleComponentTo(sc, eventingWebhook, defaultReplicas)
		common.ScaleComponentTo(sc, eventingController, defaultReplicas)
	}
	scaleDownPingSourceAdapter := func() {
		common.ScaleComponentTo(sc, pingSourceAdapter, 0)
	}

	// IMC
	imcController := types.NamespacedName{Namespace: eventingNamespace, Name: "imc-controller"}
	imcDispatcher := types.NamespacedName{Namespace: eventingNamespace, Name: "imc-dispatcher"}
	scaleUpIMC := func() {
		common.ScaleComponentTo(sc, imcController, defaultReplicas)
		common.ScaleComponentTo(sc, imcDispatcher, defaultReplicas)
	}

	// MTChannelBasedBroker
	mtBrokerController := types.NamespacedName{Namespace: eventingNamespace, Name: "mt-broker-controller"}
	mtBrokerIngress := types.NamespacedName{Namespace: eventingNamespace, Name: "mt-broker-ingress"}
	mtBrokerFilter := types.NamespacedName{Namespace: eventingNamespace, Name: "mt-broker-filter"}
	scaleUpMTChannelBasedBroker := func() {
		common.ScaleComponentTo(sc, mtBrokerController, defaultReplicas)
		common.ScaleComponentTo(sc, mtBrokerIngress, defaultReplicas)
		common.ScaleComponentTo(sc, mtBrokerFilter, defaultReplicas)
	}

	if v, err := e.apiServerSourceLister.List(labels.Everything()); err == nil && len(v) > 0 {
		scaleUpCore()
	}
	if v, err := e.pingSourceLister.List(labels.Everything()); err == nil && len(v) > 0 {
		scaleUpCore()
	} else if len(v) == 0 {
		scaleDownPingSourceAdapter()
	}
	if v, err := e.sinkBindingLister.List(labels.Everything()); err == nil && len(v) > 0 {
		scaleUpCore()
	}
	if v, err := e.containerSourceLister.List(labels.Everything()); err == nil && len(v) > 0 {
		scaleUpCore()
	}

	if v, err := e.brokerLister.List(labels.Everything()); err == nil && len(v) > 0 {
		scaleUpCore()
		scaleUpMTChannelBasedBroker() // TODO: detect which broker class the brokers are using
	}
	if v, err := e.triggerLister.List(labels.Everything()); err == nil && len(v) > 0 {
		scaleUpCore()
		scaleUpMTChannelBasedBroker() // TODO: detect which broker class the triggers are using
	}

	if v, err := e.subscriptionLister.List(labels.Everything()); err == nil && len(v) > 0 {
		scaleUpCore()
		scaleUpIMC() // TODO: detect which channel the subscriptions are using
	}
	if v, err := e.channelLister.List(labels.Everything()); err == nil && len(v) > 0 {
		scaleUpCore()
		scaleUpIMC() // TODO: detect which channel the generic channels are using
	}
	if v, err := e.imcLister.List(labels.Everything()); err == nil && len(v) > 0 {
		scaleUpCore()
		scaleUpIMC()
	}

	if v, err := e.parallelLister.List(labels.Everything()); err == nil && len(v) > 0 {
		scaleUpCore()
		scaleUpIMC() // TODO: detect which channel the parallels are using
	}
	if v, err := e.sequenceLister.List(labels.Everything()); err == nil && len(v) > 0 {
		scaleUpCore()
		scaleUpIMC() // TODO: detect which channel the sequences are using
	}

	// Webhooks need to stay up all the time since otherwise create operations will be rejected.
	// (IMC controller serves webhooks).
	common.ScaleComponentTo(sc, eventingWebhook, defaultReplicas)
	common.ScaleComponentTo(sc, imcController, defaultReplicas)

	// Scale to 0, if not otherwise specified previously
	common.ScaleComponentTo(sc, eventingController, 0)
	common.ScaleComponentTo(sc, imcDispatcher, 0)
	common.ScaleComponentTo(sc, mtBrokerController, 0)
	common.ScaleComponentTo(sc, mtBrokerFilter, 0)
	common.ScaleComponentTo(sc, mtBrokerIngress, 0)

	logger.Infow("Scaled components to", zap.Any("object", ke.Spec))

	return nil
}
