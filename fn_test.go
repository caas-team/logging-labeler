package main

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
	inputv1beta1 "github.com/crossplane/logging-labeler/input/v1beta1"
)

func TestRunFunction(t *testing.T) {
	type args struct {
		ctx context.Context
		req *fnv1beta1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1beta1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"ResponseIsReturned": {
			reason: "The function should return a response with the desired state.",
			args: args{
				req: &fnv1beta1.RunFunctionRequest{
					Input: resource.MustStructObject(&inputv1beta1.Input{
						NamespaceLabel: "testLabel",
					}),
					Meta: &fnv1beta1.RequestMeta{Tag: "hello"},
					Observed: &fnv1beta1.State{
						Composite: &fnv1beta1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "caas.telekom.de/v1alpha1",
								"kind": "XLogging",
								"metadata": {
									"name": "test-logging",
									"generation": 1
								},
								"spec": {
									"claimRef": {
										"apiVersion": "caas.telekom.de/v1alpha1",
										"kind": "Logging",
										"name": "test-logging",
										"namespace": "unit-test"
									}
								}
							}`),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1beta1.RunFunctionResponse{
					Meta: &fnv1beta1.ResponseMeta{Tag: "hello", Ttl: durationpb.New(response.DefaultTTL)},
					Desired: &fnv1beta1.State{
						Resources: map[string]*fnv1beta1.Resource{
							"logging": {
								Ready: fnv1beta1.Ready_READY_TRUE,
								Resource: resource.MustStructJSON(`{
									"apiVersion": "logging.banzaicloud.io/v1beta1",
									"kind": "Logging",
									"spec": {
										"controlNamespace": "unit-test",
										"watchNamespaceSelector": {
											"matchLabels": {
												"testLabel": "test-project"
											}
										},
										"allowClusterResourcesFromAllNamespaces": false,
										"configCheck": {
											"timeoutSeconds": 0
										},
										"enableRecreateWorkloadOnImmutableFieldChange": false,
										"flowConfigCheckDisabled": false,
										"skipInvalidResources": false
									},
									"status": {
										"problemsCount": 0
									}

								}`),
							},
						},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset()
	if _, err := client.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unit-test",
			Labels: map[string]string{
				"testLabel": "test-project",
			},
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Errorf("client.CoreV1().Namespaces().Create(...): %v", err)
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {

			f := &Function{log: logging.NewNopLogger(), cs: client}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}
