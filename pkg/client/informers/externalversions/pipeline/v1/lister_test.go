/*
Copyright 2024 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ RefreshLister[v1.TaskRun, v1.TaskRunList] = (*testLister)(nil)

type testLister struct {
	getResponse        []*v1.TaskRun
	getCount           int
	listResponse       *v1.TaskRunList
	listCount          int
	getItemsCount      int
	statusIsEmptyCount int
}

// Get gets a specific TaskRun
func (t *testLister) Get(namespace, name string, opts metav1.GetOptions) (*v1.TaskRun, error) {
	defer func() { t.getCount++ }()
	if t.getCount < len(t.getResponse) {
		return t.getResponse[t.getCount], nil
	}
	return &v1.TaskRun{}, nil
}

// List lists all TaskRuns in the given namespace
func (t *testLister) List(namespace string, opts metav1.ListOptions) (*v1.TaskRunList, error) {
	t.listCount++
	return t.listResponse, nil
}

// GetItems gets the items of the list
func (t *testLister) GetItems(list *v1.TaskRunList) []v1.TaskRun {
	t.getItemsCount++
	if list == nil {
		return nil
	}
	return list.Items
}

// StatusIsEmpty checks if the run has an empty status
func (t *testLister) StatusIsEmpty(run *v1.TaskRun) bool {
	t.statusIsEmptyCount++
	return run != nil && run.Status.StartTime.IsZero()
}

var (
	namespace = "ns"
	now       = metav1.Now()
	noStatus  = &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taskrun1",
			Namespace: namespace,
		},
	}
	hasStatus = &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taskrun2",
			Namespace: namespace,
		},
		Status: v1.TaskRunStatus{
			TaskRunStatusFields: v1.TaskRunStatusFields{
				StartTime: &now,
			},
		},
	}
	noStatusItems  = []v1.TaskRun{*noStatus, *noStatus}
	hasStatusItems = []v1.TaskRun{*hasStatus, *hasStatus}
)

func TestFetchOrRefreshList_HitCache(t *testing.T) {
	// hit cache, no need to refresh
	cacheChecked := map[string]bool{namespace: true}
	lister := &testLister{
		getResponse: []*v1.TaskRun{noStatus},
		listResponse: &v1.TaskRunList{
			Items: noStatusItems,
		},
	}
	fetchOrRefreshList[v1.TaskRun, v1.TaskRunList](lister, namespace, metav1.ListOptions{}, cacheChecked)
	if lister.getCount != 0 {
		t.Errorf("expected getCount to be 0, got %d", lister.getCount)
	}
	if lister.listCount != 1 {
		t.Errorf("expected listCount to be 1, got %d", lister.listCount)
	}
}

func TestFetchOrRefreshList_MissCacheHasStatus(t *testing.T) {
	// miss cache, no status, refresh
	cacheChecked := map[string]bool{}
	lister := &testLister{
		getResponse: []*v1.TaskRun{noStatus, hasStatus},
		listResponse: &v1.TaskRunList{
			Items: hasStatusItems,
		},
	}
	fetchOrRefreshList[v1.TaskRun, v1.TaskRunList](lister, namespace, metav1.ListOptions{}, cacheChecked)
	if lister.getCount != 0 {
		t.Errorf("expected getCount to be 0, got %d", lister.getCount)
	}
	if lister.listCount != 1 {
		t.Errorf("expected listCount to be 1, got %d", lister.listCount)
	}
	if cacheChecked[namespace] != true {
		t.Errorf("expected cacheChecked to be true, got %v", cacheChecked[namespace])
	}
}

func TestFetchOrRefreshList_MissCacheNoStatus(t *testing.T) {
	// miss cache, no status, refresh
	cacheChecked := map[string]bool{}
	lister := &testLister{
		getResponse: []*v1.TaskRun{noStatus, hasStatus},
		listResponse: &v1.TaskRunList{
			Items: noStatusItems,
		},
	}
	fetchOrRefreshList[v1.TaskRun, v1.TaskRunList](lister, namespace, metav1.ListOptions{}, cacheChecked)
	if lister.getCount != 2 {
		t.Errorf("expected getCount to be 2, got %d", lister.getCount)
	}
	if lister.listCount != 2 {
		t.Errorf("expected listCount to be 2, got %d", lister.listCount)
	}
	if cacheChecked[namespace] != true {
		t.Errorf("expected cacheChecked to be true, got %v", cacheChecked[namespace])
	}
}
