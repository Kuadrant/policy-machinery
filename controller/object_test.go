package controller

import (
	"sort"
	"testing"
	"time"

	"github.com/samber/lo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestObjectsByCreationTimestamp(t *testing.T) {
	pods := []*corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "pod1", CreationTimestamp: metav1.Time{Time: time.Unix(1, 0)}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pod2", CreationTimestamp: metav1.Time{Time: time.Unix(2, 0)}}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pod3", CreationTimestamp: metav1.Time{Time: time.Unix(1, 0)}}},
	}
	objs := lo.Map(pods, func(pod *corev1.Pod, _ int) Object { return pod })
	sort.Sort(ObjectsByCreationTimestamp(objs))
	if objs[0].GetName() != "pod1" {
		t.Errorf("expected pod1, got %s", objs[0].GetName())
	}
	if objs[1].GetName() != "pod3" {
		t.Errorf("expected pod3, got %s", objs[1].GetName())
	}
	if objs[2].GetName() != "pod2" {
		t.Errorf("expected pod2, got %s", objs[2].GetName())
	}
}

func TestObjectAs(t *testing.T) {
	var obj Object = &corev1.Pod{}
	if ObjectAs[*corev1.Pod](obj, 0) == nil {
		t.Errorf("expected *corev1.Pod, got nil")
	}
}
