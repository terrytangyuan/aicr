// Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/NVIDIA/aicr/pkg/defaults"
	"github.com/NVIDIA/aicr/pkg/errors"
	podutil "github.com/NVIDIA/aicr/pkg/k8s/pod"
	"github.com/NVIDIA/aicr/validators"
	"github.com/NVIDIA/aicr/validators/helper"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	nvidiaSMIPodTemplateFile = "testdata/nvidia-smi-verify-pod.yaml"
	gpuCheckSuccessMsg       = "GPU_CHECK_SUCCESS"
	nvidiaSMILogContextLines = 20
)

// checkNvidiaSMI verifies that nvidia-smi works correctly on all GPU nodes.
func checkNvidiaSMI(ctx *validators.Context) error {
	gpuNodes, err := helper.FindSchedulableGpuNodes(ctx.Ctx, ctx.Clientset)
	if err != nil {
		return errors.Wrap(errors.ErrCodeInternal, "failed to query for GPU nodes", err)
	}

	if len(gpuNodes) == 0 {
		return validators.Skip("no GPU nodes found in the cluster")
	}

	fmt.Printf("Found %d GPU node(s):\n", len(gpuNodes))
	for _, node := range gpuNodes {
		fmt.Printf("  %s\n", node.Name)
	}

	// Check if any nodes are busy
	var busyNodes []string
	for _, node := range gpuNodes {
		busy, busyErr := helper.IsNodeGpuBusy(ctx.Ctx, ctx.Clientset, node.Name)
		if busyErr != nil {
			slog.Warn("error checking busy status, treating as busy", "node", node.Name, "error", busyErr)
			busyNodes = append(busyNodes, node.Name)
			continue
		}
		if busy {
			busyNodes = append(busyNodes, node.Name)
		}
	}

	if len(busyNodes) > 0 {
		return validators.Skip(fmt.Sprintf("GPU nodes busy with existing workloads: %v", busyNodes))
	}

	fmt.Printf("All %d GPU node(s) available. Verifying...\n", len(gpuNodes))

	// Verify each GPU node
	results := make(map[string]error)
	for _, node := range gpuNodes {
		slog.Info("verifying node", "node", node.Name)
		if verifyErr := verifySingleGPUNode(ctx, node.Name); verifyErr != nil {
			results[node.Name] = verifyErr
			fmt.Printf("  %s: FAILED (%v)\n", node.Name, verifyErr)
		} else {
			results[node.Name] = nil
			fmt.Printf("  %s: OK\n", node.Name)
		}
	}

	// Report results
	var failedNodes []string
	for nodeName, nodeErr := range results {
		if nodeErr != nil {
			failedNodes = append(failedNodes, fmt.Sprintf("%s (%v)", nodeName, nodeErr))
		}
	}

	if len(failedNodes) > 0 {
		return errors.New(errors.ErrCodeInternal,
			fmt.Sprintf("GPU verification failed on %d/%d nodes: %v",
				len(failedNodes), len(gpuNodes), failedNodes))
	}

	fmt.Printf("Successfully verified GPU on all %d nodes\n", len(gpuNodes))
	return nil
}

func verifySingleGPUNode(ctx *validators.Context, nodeName string) error {
	templateData := map[string]string{
		"NODE_NAME": strings.ToLower(nodeName),
		"NAMESPACE": ctx.Namespace,
		"IMAGE":     getNvidiaSMIImage(ctx),
	}

	// Load and create pod directly — no PodLifecycle wrapper needed.
	pod, err := helper.LoadPodFromTemplate(nvidiaSMIPodTemplateFile, templateData)
	if err != nil {
		return errors.Wrap(errors.ErrCodeInternal, "failed to load pod template", err)
	}

	createdPod, err := ctx.Clientset.CoreV1().Pods(ctx.Namespace).Create(ctx.Ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(errors.ErrCodeInternal, "failed to create verification pod", err)
	}

	defer func() { //nolint:contextcheck // Fresh context: parent may be canceled during cleanup
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), defaults.K8sCleanupTimeout)
		defer cleanupCancel()
		if cleanupErr := ctx.Clientset.CoreV1().Pods(ctx.Namespace).Delete(cleanupCtx, createdPod.Name, metav1.DeleteOptions{}); cleanupErr != nil {
			slog.Warn("failed to cleanup pod", "namespace", createdPod.Namespace, "pod", createdPod.Name, "error", cleanupErr)
		}
	}()

	// Use pkg/k8s/pod utilities directly.
	waitErr := podutil.WaitForPodSucceeded(ctx.Ctx, ctx.Clientset, ctx.Namespace, createdPod.Name, defaults.PodWaitTimeout)

	podLogs, logErr := podutil.GetPodLogs(ctx.Ctx, ctx.Clientset, ctx.Namespace, createdPod.Name, "")
	if logErr != nil {
		slog.Warn("failed to get logs for pod", "node", nodeName, "error", logErr)
		podLogs = fmt.Sprintf("failed to retrieve pod logs: %v", logErr)
	}

	if waitErr != nil {
		logSnippet := getLogSnippet(podLogs, nvidiaSMILogContextLines)
		return errors.Wrap(errors.ErrCodeInternal,
			fmt.Sprintf("pod failed on node %s\nFirst %d lines:\n%s",
				nodeName, nvidiaSMILogContextLines, logSnippet), waitErr)
	}

	return verifyNvidiaSMILogs(podLogs, createdPod)
}

func getLogSnippet(logs string, maxLines int) string {
	lines := strings.Split(logs, "\n")
	if len(lines) <= maxLines {
		return logs
	}
	return strings.Join(lines[:maxLines], "\n")
}

func verifyNvidiaSMILogs(podLogs string, pod *v1.Pod) error {
	requiredStrings := []string{
		"NVIDIA-SMI",
		"Driver Version:",
		"CUDA Version:",
		gpuCheckSuccessMsg,
	}

	var missing []string
	for _, required := range requiredStrings {
		if !strings.Contains(podLogs, required) {
			missing = append(missing, required)
		}
	}

	if len(missing) > 0 {
		return errors.New(errors.ErrCodeInternal,
			fmt.Sprintf("log verification failed for pod %s/%s: missing %v",
				pod.Namespace, pod.Name, missing))
	}

	return nil
}

func getNvidiaSMIImage(_ *validators.Context) string {
	// Default image for most GPU types.
	// Future: read accelerator type from recipe to select GB200-specific image.
	return "nvcr.io/nvidia/cuda:13.0.0-base-ubuntu22.04"
}
