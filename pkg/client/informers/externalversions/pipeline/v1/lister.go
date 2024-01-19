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
	"context"
	time "time"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	versioned "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/logging"
)

// When the crds are just updated, the data in the apiserver cache is still v1beta1.
// If you get it directly, you will encounter the problem of status loss.
// At this time, you can force the apiserver cache to be refreshed by setting ResourceVersion to empty

// RefreshLister is an interface for listing objects
type RefreshLister[T any, L any] interface {
	// List lists all object in the given namespace
	List(namespace string, opts metav1.ListOptions) (*L, error)

	// Get gets a specific object
	Get(namespace, name string, opts metav1.GetOptions) (*T, error)

	// GetItems gets the items of the list
	GetItems(*L) []T

	// StatusIsEmpty checks if the run has an empty status
	StatusIsEmpty(*T) bool
}

var _ RefreshLister[v1.TaskRun, v1.TaskRunList] = (*TaskRunLister)(nil)

// TaskRunLister is an interface for listing Tekton TaskRuns.
type TaskRunLister struct {
	Client versioned.Interface
}

// List lists all TaskRuns in the given namespace
func (t *TaskRunLister) List(namespace string, opts metav1.ListOptions) (*v1.TaskRunList, error) {
	return t.Client.TektonV1().TaskRuns(namespace).List(context.TODO(), opts)
}

// Get gets a specific TaskRun
func (t *TaskRunLister) Get(namespace, name string, opts metav1.GetOptions) (*v1.TaskRun, error) {
	return t.Client.TektonV1().TaskRuns(namespace).Get(context.TODO(), name, opts)
}

// GetItems gets the items of the list
func (t *TaskRunLister) GetItems(list *v1.TaskRunList) []v1.TaskRun {
	if list == nil {
		return nil
	}
	return list.Items
}

// StatusIsEmpty checks if the run has an empty status
func (t *TaskRunLister) StatusIsEmpty(run *v1.TaskRun) bool {
	return run != nil && run.Status.StartTime.IsZero()
}

// PipelineRunLister is an interface for listing Tekton PipelineRuns.
var _ RefreshLister[v1.PipelineRun, v1.PipelineRunList] = (*PipelineRunLister)(nil)

type PipelineRunLister struct {
	Client versioned.Interface
}

// List lists all PipelineRuns in the given namespace
func (p *PipelineRunLister) List(namespace string, opts metav1.ListOptions) (*v1.PipelineRunList, error) {
	return p.Client.TektonV1().PipelineRuns(namespace).List(context.TODO(), opts)
}

// Get gets a specific PipelineRun
func (p *PipelineRunLister) Get(namespace, name string, opts metav1.GetOptions) (*v1.PipelineRun, error) {
	return p.Client.TektonV1().PipelineRuns(namespace).Get(context.TODO(), name, opts)
}

// GetItems gets the items of the list
func (p *PipelineRunLister) GetItems(list *v1.PipelineRunList) []v1.PipelineRun {
	if list == nil {
		return nil
	}
	return list.Items
}

// StatusIsEmpty checks if the run has an empty status
func (p *PipelineRunLister) StatusIsEmpty(run *v1.PipelineRun) bool {
	return run != nil && run.Status.StartTime.IsZero()
}

// needRefresh checks if the run needs to be refreshed
// If the status in etcd is empty, but the status in apiserver is not empty,
// it means that the cache in etcd needs to be refreshed
func needRefresh[T any, L any](_ context.Context, lister RefreshLister[T, L], item *T) (bool, error) {
	if !lister.StatusIsEmpty(item) {
		// if the status in apiserver is not empty, no need to refresh
		return false, nil
	}
	run := interface{}(item).(metav1.Object)
	// use direct client to get the item from apiserver
	item, err := lister.Get(run.GetNamespace(), run.GetName(), metav1.GetOptions{})
	if err != nil {
		// even if the not found error is returned, the cache still needs to be refreshed
		return true, err
	}
	return !lister.StatusIsEmpty(item), nil
}

// fetchOrRefreshList fetches the list from the client or refreshes the cache if needed.
func fetchOrRefreshList[T any, L any](lister RefreshLister[T, L],
	namespace string, options metav1.ListOptions, cacheChecked map[string]bool) (runtime.Object, error) {
	ctx := context.TODO()
	logger := logging.FromContext(ctx).With("ns", namespace)

	// Retrieve the list of object from the client.
	runs, err := lister.List(namespace, options)
	if err != nil {
		return nil, err
	}

	if cacheChecked[namespace] {
		// if the namespace has been checked, no need to refresh
		return interface{}(runs).(runtime.Object), err
	}

	var refreshNeeded bool
	items := lister.GetItems(runs)
	for i := range items {
		item := &items[i]
		if refresh, refreshErr := needRefresh[T, L](ctx, lister, item); refresh || refreshErr != nil {
			refreshNeeded = true
			meta := interface{}(item).(metav1.Object)
			obj := interface{}(item).(runtime.Object)
			logger.Infow("Detected a need to refresh cache", "options", options,
				"gvk", obj.GetObjectKind().GroupVersionKind().String(),
				"name", meta.GetName(), "namespace", meta.GetNamespace(), "error", refreshErr)
			break
		}
	}
	// Refresh the list if needed.
	if refreshNeeded {
		ops := options.DeepCopy()
		ops.ResourceVersion = "" // Clear ResourceVersion to fetch a new list without using the cache.
		startTime := time.Now()
		runs, err = lister.List(namespace, *ops)
		logger.Infow("Refreshed the cache", "duration", time.Since(startTime), "error", err)
	}

	if err == nil {
		// if the list is fetched successfully or no need refresh, mark the namespace as checked
		cacheChecked[namespace] = true
	}

	return interface{}(runs).(runtime.Object), err
}
