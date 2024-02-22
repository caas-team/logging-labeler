package main

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/response"
	inputv1beta1 "github.com/crossplane/logging-labeler/input/v1beta1"
	"github.com/kube-logging/logging-operator/pkg/sdk/logging/api/v1beta1"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer
	cs  kubernetes.Interface
	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(ctx context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	rsp := response.To(req, response.DefaultTTL)
	in := &inputv1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get input from %T", req))
		return rsp, nil
	}

	xr, err := request.GetObservedCompositeResource(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get observed composed resources"))
		return rsp, nil
	}

	desired, err := request.GetDesiredComposedResources(req)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get desired composed resources"))
		return rsp, nil
	}

	if err := v1beta1.AddToScheme(composed.Scheme); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot add to scheme"))
		return rsp, nil
	}

	ns := xr.Resource.GetClaimReference().Namespace

	targetns, err := f.cs.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get namespace %s", ns))
		return rsp, nil
	}

	projectid, ok := targetns.GetLabels()[in.NamespaceLabel]
	if !ok {
		response.Fatal(rsp, fmt.Errorf("cannot get project id %s for namespace %s", projectid, targetns.GetName()))
		return rsp, nil
	}

	l := &v1beta1.Logging{}
	l.Spec.ControlNamespace = ns
	l.Spec.WatchNamespaceSelector = &metav1.LabelSelector{}
	l.Spec.WatchNamespaceSelector.MatchLabels = map[string]string{in.NamespaceLabel: projectid}

	cd, err := composed.From(l)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot convert %T to %T", l, &composed.Unstructured{}))
		return rsp, nil
	}

	desired["logging"] = &resource.DesiredComposed{Resource: cd}

	f.log.Info("Desired composed resources", "desired", desired)

	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot set desired composed resources"))
		return rsp, nil
	}

	return rsp, nil
}
