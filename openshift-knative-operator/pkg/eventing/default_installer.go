package eventing

import (
	"context"
	"os"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/operator/pkg/apis/operator/v1beta1"
	operatorv1beta1 "knative.dev/operator/pkg/apis/operator/v1beta1"
	operatorclient "knative.dev/operator/pkg/client/injection/client"
	"knative.dev/pkg/logging"
)

func NewDefaultInstaller(ctx context.Context) error {

	go createDefaultKnativeEventingInstance(ctx)

	return nil
}

func createDefaultKnativeEventingInstance(ctx context.Context) {
	eventingNamespace := os.Getenv(requiredNsEnvName)

	defaultKnativeEventingInstance := &operatorv1beta1.KnativeEventing{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: eventingNamespace,
		},
		Spec: v1beta1.KnativeEventingSpec{},
	}

	logger := logging.FromContext(ctx).With(zap.String("component", "installer"))
	client := operatorclient.Get(ctx).OperatorV1beta1()

	createInstance := func() (bool, error) {
		instances, err := client.KnativeEventings(eventingNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logger.Debugw("Failed to list instances, it will retry", zap.Error(err))
			return false, nil
		}

		if len(instances.Items) > 0 {
			return true, nil
		}

		_, err = client.KnativeEventings(eventingNamespace).Create(ctx, defaultKnativeEventingInstance, metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) || apierrors.IsConflict(err) {
				return true, nil
			}

			logger.Debugw("Failed to create default instance, it will retry", zap.Error(err))
		}

		return true, nil
	}

	if err := wait.PollUntil(5*time.Second, createInstance, ctx.Done()); err != nil {
		logger.Warnw("Failed to create default instance", zap.Error(err))
	}
}
