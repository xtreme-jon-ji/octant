/*
Copyright (c) 2019 VMware, Inc. All Rights Reserved.
SPDX-License-Identifier: Apache-2.0
*/

package objectvisitor

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vmware/octant/internal/gvk"
	queryerFake "github.com/vmware/octant/internal/queryer/fake"
	tu "github.com/vmware/octant/internal/testutil"
)

func Test_DefaultVisitor_Visit(t *testing.T) {
	type mocks struct {
		q *queryerFake.MockQueryer
	}

	cases := []struct {
		name            string
		init            func(t *testing.T, m *mocks) []ClusterObject
		expectedObjects []string
		expectedEdges   map[string][]string
	}{
		{
			name: "workload with pod",
			init: func(t *testing.T, m *mocks) []ClusterObject {
				daemonSet := tu.CreateDaemonSet("daemonset")
				pod := tu.CreatePod("pod")
				pod.SetOwnerReferences(toOwnerReferences(t, daemonSet))

				expectChildren(t, m.q, daemonSet, []runtime.Object{tu.ToUnstructured(t, pod)}, nil)

				m.q.EXPECT().
					ServicesForPod(gomock.Any(), gomock.Eq(pod)).
					Return([]*corev1.Service{}, nil).AnyTimes()

				m.q.EXPECT().
					OwnerReference(gomock.Any(), gomock.Eq("namespace"), gomock.Eq(pod.OwnerReferences[0])).
					Return(daemonSet, nil).AnyTimes()

				expectChildren(t, m.q, pod, []runtime.Object{}, nil)

				return []ClusterObject{daemonSet, pod}
			},
			expectedObjects: []string{
				"apps/v1, Kind=DaemonSet:daemonset",
				"/v1, Kind=Pod:pod",
			},
			expectedEdges: map[string][]string{
				"daemonset": {"pod"},
			},
		},
		{
			name: "service with pod",
			init: func(t *testing.T, m *mocks) []ClusterObject {
				service := tu.CreateService("service")
				pod := tu.CreatePod("pod")

				m.q.EXPECT().
					PodsForService(gomock.Any(), gomock.Eq(service)).
					Return([]*corev1.Pod{pod}, nil).AnyTimes()

				m.q.EXPECT().
					ServicesForPod(gomock.Any(), gomock.Eq(pod)).
					Return([]*corev1.Service{service}, nil).AnyTimes()

				expectChildren(t, m.q, pod, []runtime.Object{}, nil)
				expectChildren(t, m.q, service, []runtime.Object{}, nil)

				m.q.EXPECT().
					IngressesForService(gomock.Any(), gomock.Eq(service)).
					Return([]*extv1beta1.Ingress{}, nil).AnyTimes()

				return []ClusterObject{service}
			},
			expectedObjects: []string{
				"/v1, Kind=Service:service",
				"/v1, Kind=Pod:pod",
			},
			expectedEdges: map[string][]string{
				"service": {"pod"},
			},
		},
		{
			name: "ingress with service and pod",
			init: func(t *testing.T, m *mocks) []ClusterObject {
				ingress := tu.CreateIngress("ingress")
				service := tu.CreateService("service")
				pod := tu.CreatePod("pod")

				m.q.EXPECT().
					ServicesForIngress(gomock.Any(), gomock.Eq(ingress)).
					Return([]*corev1.Service{service}, nil).AnyTimes()

				m.q.EXPECT().
					IngressesForService(gomock.Any(), gomock.Eq(service)).
					Return([]*extv1beta1.Ingress{ingress}, nil).AnyTimes()

				m.q.EXPECT().
					PodsForService(gomock.Any(), gomock.Eq(service)).
					Return([]*corev1.Pod{pod}, nil).AnyTimes()

				m.q.EXPECT().
					ServicesForPod(gomock.Any(), gomock.Eq(pod)).
					Return([]*corev1.Service{service}, nil).AnyTimes()

				expectChildren(t, m.q, ingress, []runtime.Object{}, nil)
				expectChildren(t, m.q, service, []runtime.Object{}, nil)
				expectChildren(t, m.q, pod, []runtime.Object{}, nil)

				return []ClusterObject{ingress, service, pod}
			},
			expectedObjects: []string{
				"extensions/v1beta1, Kind=Ingress:ingress",
				"/v1, Kind=Pod:pod",
				"/v1, Kind=Service:service",
			},
			expectedEdges: map[string][]string{
				"ingress": {"service"},
				"service": {"pod"},
			},
		},
		{
			name: "full workload",
			init: func(t *testing.T, m *mocks) []ClusterObject {
				ingress := tu.CreateIngress("ingress")
				service := tu.CreateService("service")
				pod := tu.CreatePod("pod")
				deployment := tu.CreateDeployment("deployment")
				replicaSet := tu.CreateReplicaSet("replicaSet")
				serviceAccount := tu.CreateServiceAccount("service-account")

				replicaSet.SetOwnerReferences(toOwnerReferences(t, deployment))
				pod.SetOwnerReferences(toOwnerReferences(t, replicaSet))
				pod.Spec.ServiceAccountName = "service-account"

				m.q.EXPECT().
					ServicesForIngress(gomock.Any(), gomock.Eq(ingress)).
					Return([]*corev1.Service{service}, nil).AnyTimes()

				m.q.EXPECT().
					IngressesForService(gomock.Any(), gomock.Eq(service)).
					Return([]*extv1beta1.Ingress{ingress}, nil).AnyTimes()

				m.q.EXPECT().
					PodsForService(gomock.Any(), gomock.Eq(service)).
					Return([]*corev1.Pod{pod}, nil).AnyTimes()

				m.q.EXPECT().
					ServicesForPod(gomock.Any(), gomock.Eq(pod)).
					Return([]*corev1.Service{service}, nil).AnyTimes()

				m.q.EXPECT().
					ServiceAccountForPod(gomock.Any(), gomock.Eq(pod)).
					Return(serviceAccount, nil).AnyTimes()

				expectChildren(t, m.q, ingress, []runtime.Object{}, nil)
				expectChildren(t, m.q, service, []runtime.Object{}, nil)
				expectChildren(t, m.q, pod, []runtime.Object{}, nil)
				expectChildren(t, m.q, replicaSet, []runtime.Object{tu.ToUnstructured(t, pod)}, nil)
				expectChildren(t, m.q, deployment, []runtime.Object{tu.ToUnstructured(t, replicaSet)}, nil)
				expectChildren(t, m.q, serviceAccount, []runtime.Object{}, nil)

				m.q.EXPECT().
					OwnerReference(gomock.Any(), gomock.Eq("namespace"), gomock.Eq(pod.OwnerReferences[0])).
					Return(replicaSet, nil).AnyTimes()

				m.q.EXPECT().
					OwnerReference(gomock.Any(), gomock.Eq("namespace"), gomock.Eq(replicaSet.OwnerReferences[0])).
					Return(deployment, nil).AnyTimes()

				return []ClusterObject{ingress, service, pod, replicaSet, deployment}
			},
			expectedObjects: []string{
				"extensions/v1beta1, Kind=Ingress:ingress",
				"/v1, Kind=Pod:pod",
				"/v1, Kind=Service:service",
				"/v1, Kind=ServiceAccount:service-account",
				"apps/v1, Kind=ReplicaSet:replicaSet",
				"apps/v1, Kind=Deployment:deployment",
			},
			expectedEdges: map[string][]string{
				"service":    {"pod"},
				"replicaSet": {"pod"},
				"ingress":    {"service"},
				"deployment": {"replicaSet"},
				"pod":        {"service-account"},
			},
		},
		{
			name: "multiple workloads/services, single ingress",
			init: func(t *testing.T, m *mocks) []ClusterObject {
				d1 := tu.CreateDeployment("d1")
				d1rs1 := tu.CreateReplicaSet("d1rs1")
				d1rs1.SetOwnerReferences(toOwnerReferences(t, d1))
				d1rs1p1 := tu.CreatePod("d1rs1p1")
				d1rs1p1.SetOwnerReferences(toOwnerReferences(t, d1rs1))
				d1rs1p2 := tu.CreatePod("d1rs1p2")
				d1rs1p2.SetOwnerReferences(toOwnerReferences(t, d1rs1))
				s1 := tu.CreateService("s1")

				d2 := tu.CreateDeployment("d2")
				d2rs1 := tu.CreateReplicaSet("d2rs1")
				d2rs1.SetOwnerReferences(toOwnerReferences(t, d2))
				d2rs1p1 := tu.CreatePod("d2rs1p1")
				d2rs1p1.SetOwnerReferences(toOwnerReferences(t, d2rs1))
				s2 := tu.CreateService("s2")

				ingress := tu.CreateIngress("i1")

				expectChildren(t, m.q, d1, []runtime.Object{tu.ToUnstructured(t, d1rs1)}, nil)
				expectChildren(t, m.q, d1rs1, []runtime.Object{tu.ToUnstructured(t, d1rs1p1), tu.ToUnstructured(t, d1rs1p2)}, nil)
				expectChildren(t, m.q, d1rs1p1, []runtime.Object{}, nil)
				expectChildren(t, m.q, d1rs1p2, []runtime.Object{}, nil)
				expectChildren(t, m.q, d2, []runtime.Object{tu.ToUnstructured(t, d2rs1)}, nil)
				expectChildren(t, m.q, d2rs1, []runtime.Object{tu.ToUnstructured(t, d2rs1p1)}, nil)
				expectChildren(t, m.q, d2rs1p1, []runtime.Object{}, nil)
				expectChildren(t, m.q, s1, []runtime.Object{}, nil)
				expectChildren(t, m.q, s2, []runtime.Object{}, nil)
				expectChildren(t, m.q, ingress, []runtime.Object{}, nil)

				m.q.EXPECT().
					OwnerReference(gomock.Any(), gomock.Eq("namespace"), gomock.Eq(d1rs1.OwnerReferences[0])).
					Return(d1, nil).AnyTimes()

				m.q.EXPECT().
					OwnerReference(gomock.Any(), gomock.Eq("namespace"), gomock.Eq(d2rs1.OwnerReferences[0])).
					Return(d2, nil).AnyTimes()

				m.q.EXPECT().
					OwnerReference(gomock.Any(), gomock.Eq("namespace"), gomock.Eq(d1rs1p1.OwnerReferences[0])).
					Return(d1rs1, nil).AnyTimes()

				m.q.EXPECT().
					OwnerReference(gomock.Any(), gomock.Eq("namespace"), gomock.Eq(d1rs1p2.OwnerReferences[0])).
					Return(d1rs1, nil).AnyTimes()

				m.q.EXPECT().
					OwnerReference(gomock.Any(), gomock.Eq("namespace"), gomock.Eq(d2rs1p1.OwnerReferences[0])).
					Return(d2rs1, nil).AnyTimes()

				m.q.EXPECT().
					ServicesForPod(gomock.Any(), gomock.Eq(d1rs1p1)).
					Return([]*corev1.Service{s1}, nil).AnyTimes()

				m.q.EXPECT().
					ServicesForPod(gomock.Any(), gomock.Eq(d1rs1p2)).
					Return([]*corev1.Service{s1}, nil).AnyTimes()

				m.q.EXPECT().
					ServicesForPod(gomock.Any(), gomock.Eq(d2rs1p1)).
					Return([]*corev1.Service{s2}, nil).AnyTimes()

				m.q.EXPECT().
					PodsForService(gomock.Any(), gomock.Eq(s1)).
					Return([]*corev1.Pod{d1rs1p1, d1rs1p2}, nil).AnyTimes()

				m.q.EXPECT().
					PodsForService(gomock.Any(), gomock.Eq(s2)).
					Return([]*corev1.Pod{d2rs1p1}, nil).AnyTimes()

				m.q.EXPECT().
					IngressesForService(gomock.Any(), gomock.Eq(s1)).
					Return([]*extv1beta1.Ingress{ingress}, nil).AnyTimes()

				m.q.EXPECT().
					IngressesForService(gomock.Any(), gomock.Eq(s2)).
					Return([]*extv1beta1.Ingress{ingress}, nil).AnyTimes()

				m.q.EXPECT().
					ServicesForIngress(gomock.Any(), gomock.Eq(ingress)).
					Return([]*corev1.Service{s1, s2}, nil).AnyTimes()

				return []ClusterObject{d1, d1rs1, d1rs1p1, d1rs1p2, d2, d2rs1,
					d2rs1p1, s1, s2, ingress}
			},
			expectedObjects: []string{
				"apps/v1, Kind=Deployment:d1",
				"apps/v1, Kind=ReplicaSet:d1rs1",
				"/v1, Kind=Pod:d1rs1p1",
				"/v1, Kind=Pod:d1rs1p2",
				"apps/v1, Kind=Deployment:d2",
				"apps/v1, Kind=ReplicaSet:d2rs1",
				"/v1, Kind=Pod:d2rs1p1",
				"/v1, Kind=Service:s1",
				"/v1, Kind=Service:s2",
				"extensions/v1beta1, Kind=Ingress:i1",
			},
			expectedEdges: map[string][]string{
				"d1":    {"d1rs1"},
				"d1rs1": {"d1rs1p1", "d1rs1p2"},
				"s1":    {"d1rs1p1", "d1rs1p2"},
				"d2":    {"d2rs1"},
				"d2rs1": {"d2rs1p1"},
				"s2":    {"d2rs1p1"},
				"i1":    {"s1", "s2"},
			},
		},
	}

	gvks := []schema.GroupVersionKind{gvk.DaemonSetGVK, gvk.DeploymentGVK, gvk.IngressGVK, gvk.PodGVK,
		gvk.ServiceGVK, gvk.ReplicaSetGVK, gvk.ReplicationControllerGVK, gvk.StatefulSetGVK, gvk.ServiceAccountGVK}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := &mocks{
				q: queryerFake.NewMockQueryer(ctrl),
			}

			require.NotNil(t, tc.init, "init func is required")

			objects := tc.init(t, m)

			for _, object := range objects {
				t.Run(fmt.Sprintf("seeded with %T", object), func(t *testing.T) {
					factoryGen := NewDefaultFactoryGenerator()

					ic := identityCollector{t: t}

					for _, groupVersionKinds := range gvks {
						factoryRegister(t, factoryGen, groupVersionKinds, ic.factoryFn)
					}

					dv, err := NewDefaultVisitor(m.q, factoryGen.FactoryFunc())
					require.NoError(t, err)

					ctx := context.Background()

					err = dv.Visit(ctx, object)
					require.NoError(t, err)

					ic.assertMatch(tc.expectedObjects)
					ic.assertChildren(tc.expectedEdges)
				})
			}
		})
	}
}

func toOwnerReferences(t *testing.T, object ClusterObject) []metav1.OwnerReference {
	objectKind := object.GetObjectKind()
	apiVersion, kind := objectKind.GroupVersionKind().ToAPIVersionAndKind()

	accessor := meta.NewAccessor()
	name, err := accessor.Name(object)
	require.NoError(t, err)

	uid, err := accessor.UID(object)
	require.NoError(t, err)

	return []metav1.OwnerReference{
		{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       name,
			UID:        uid,
		},
	}
}

func expectChildren(t *testing.T, q *queryerFake.MockQueryer, object runtime.Object, found ...interface{}) {
	q.EXPECT().
		Children(gomock.Any(), gomock.Eq(tu.ToUnstructured(t, object))).
		Return(found...).AnyTimes()
}

func factoryRegister(
	t *testing.T,
	gen *DefaultFactoryGenerator,
	gvk schema.GroupVersionKind,
	factory ObjectHandlerFactory) {
	err := gen.Register(gvk, factory)
	require.NoError(t, err)
}

type testObject struct {
	processFn  func(ctx context.Context, object ClusterObject) error
	addChildFn func(parent ClusterObject, children ...ClusterObject) error
	mu         sync.Mutex
}

func (o *testObject) Process(ctx context.Context, object ClusterObject) error {
	return o.processFn(ctx, object)
}

func (o *testObject) AddChild(parent ClusterObject, children ...ClusterObject) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.addChildFn(parent, children...)
}

type identityCollector struct {
	t           *testing.T
	gotVisits   []string
	gotChildren map[string][]string

	o *testObject

	mu sync.Mutex
}

func (ic *identityCollector) factoryFn(object ClusterObject) (ObjectHandler, error) {
	if ic.o == nil {
		ic.gotChildren = make(map[string][]string)

		objectKind := object.GetObjectKind()
		if objectKind == nil {
			return nil, errors.Errorf("object kind is nil")
		}

		accessor := meta.NewAccessor()

		ic.o = &testObject{
			processFn: func(ctx context.Context, clusterObject ClusterObject) error {
				ic.mu.Lock()
				defer ic.mu.Unlock()

				name, err := accessor.Name(clusterObject)
				if err != nil {
					return err
				}

				apiVersion, err := accessor.APIVersion(clusterObject)
				if err != nil {
					return err
				}

				kind, err := accessor.Kind(clusterObject)
				if err != nil {
					return err
				}

				groupVersionKind := schema.FromAPIVersionAndKind(apiVersion, kind)

				ic.gotVisits = append(ic.gotVisits,
					fmt.Sprintf("%s:%s", groupVersionKind, name))
				return nil
			},
			addChildFn: func(parent ClusterObject, children ...ClusterObject) error {
				ic.mu.Lock()
				defer ic.mu.Unlock()

				parentUID, err := accessor.UID(parent)
				if err != nil {
					return err
				}

				pUID := string(parentUID)

				for _, child := range children {
					childUID, err := accessor.UID(child)
					if err != nil {
						return err
					}

					cUID := string(childUID)
					ic.gotChildren[pUID] = append(ic.gotChildren[pUID], cUID)
				}
				return nil
			},
		}
	}

	return ic.o, nil
}

func (ic *identityCollector) assertMatch(expected []string) {
	got := ic.gotVisits

	sort.Strings(expected)
	sort.Strings(got)

	assert.Equal(ic.t, expected, got)
}

func (ic *identityCollector) assertChildren(expected map[string][]string) {
	got := ic.gotChildren
	for k := range expected {
		sort.Strings(expected[k])
	}
	for k := range got {
		sort.Strings(got[k])
	}

	assert.Equal(ic.t, expected, got, "children did not match")
}