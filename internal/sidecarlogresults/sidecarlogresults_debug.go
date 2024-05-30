/*
Copyright 2019 The Tekton Authors

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

package sidecarlogresults

import (
	"context"
	"io"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// DebugSidecarLogs show the logs of the sidecar container
func DebugSidecarLogs(ctx context.Context, logger *zap.SugaredLogger, clientset kubernetes.Interface, namespace string, name string, container string, podPhase corev1.PodPhase) {
	logger = logger.With("pod", name).With("ns", namespace)
	if podPhase == corev1.PodPending {
		logger.Infow("pod is in pending state, skipping sidecar logs")
		return
	}
	podLogOpts := corev1.PodLogOptions{Container: container}
	req := clientset.CoreV1().Pods(namespace).GetLogs(name, &podLogOpts)
	sidecarLogs, err := req.Stream(ctx)
	if err != nil {
		logger.Errorw("error getting sidecar logs", "error", err)
		return
	}
	defer sidecarLogs.Close()

	data, err := io.ReadAll(sidecarLogs)
	if err != nil {
		logger.Errorw("error reading sidecar logs", "error", err)
	}

	logger.Infow("sidecar logs", "logs", string(data))
}
