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

package validator

import (
	"context"
	"testing"

	"github.com/NVIDIA/aicr/pkg/validator/catalog"
	"github.com/NVIDIA/aicr/pkg/validator/ctrf"
	corev1 "k8s.io/api/core/v1"
)

func TestNewDefaults(t *testing.T) {
	v := New()

	if v.Namespace != "aicr-validation" {
		t.Errorf("Namespace = %q, want %q", v.Namespace, "aicr-validation")
	}
	if v.RunID == "" {
		t.Error("RunID should be generated")
	}
	if !v.Cleanup {
		t.Error("Cleanup should default to true")
	}
	if v.NoCluster {
		t.Error("NoCluster should default to false")
	}
	if len(v.Tolerations) != 1 || v.Tolerations[0].Operator != corev1.TolerationOpExists {
		t.Errorf("Tolerations should default to tolerate-all, got %v", v.Tolerations)
	}
}

func TestNewWithOptions(t *testing.T) {
	v := New(
		WithVersion("1.0.0"),
		WithNamespace("custom-ns"),
		WithRunID("test-run"),
		WithCleanup(false),
		WithNoCluster(true),
		WithImagePullSecrets([]string{"secret1"}),
		WithTolerations(nil),
	)

	if v.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", v.Version, "1.0.0")
	}
	if v.Namespace != "custom-ns" {
		t.Errorf("Namespace = %q, want %q", v.Namespace, "custom-ns")
	}
	if v.RunID != "test-run" {
		t.Errorf("RunID = %q, want %q", v.RunID, "test-run")
	}
	if v.Cleanup {
		t.Error("Cleanup should be false")
	}
	if !v.NoCluster {
		t.Error("NoCluster should be true")
	}
	if len(v.ImagePullSecrets) != 1 || v.ImagePullSecrets[0] != "secret1" {
		t.Errorf("ImagePullSecrets = %v", v.ImagePullSecrets)
	}
}

func TestGenerateRunID(t *testing.T) {
	id1 := generateRunID()
	id2 := generateRunID()

	if id1 == "" {
		t.Error("RunID should not be empty")
	}
	if id1 == id2 {
		t.Error("RunIDs should be unique")
	}
	if len(id1) < 20 {
		t.Errorf("RunID too short: %q", id1)
	}
}

func loadEmbeddedCatalog(t *testing.T) *catalog.ValidatorCatalog {
	t.Helper()
	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("failed to load catalog: %v", err)
	}
	return cat
}

func TestPhasesSkipped(t *testing.T) {
	v := New(WithVersion("1.0.0"))
	cat := loadEmbeddedCatalog(t)

	results := v.phasesSkipped(cat, PhaseOrder, "test reason")
	if len(results) != len(PhaseOrder) {
		t.Fatalf("expected %d results, got %d", len(PhaseOrder), len(results))
	}

	for i, pr := range results {
		if pr.Phase != PhaseOrder[i] {
			t.Errorf("results[%d].Phase = %q, want %q", i, pr.Phase, PhaseOrder[i])
		}
		if pr.Status != ctrf.StatusSkipped {
			t.Errorf("results[%d].Status = %q, want %q", i, pr.Status, ctrf.StatusSkipped)
		}
		if pr.Report == nil {
			t.Errorf("results[%d].Report should not be nil", i)
		}
	}
}

func TestPhasesSkippedSubset(t *testing.T) {
	v := New(WithVersion("1.0.0"))
	cat := loadEmbeddedCatalog(t)

	subset := []Phase{PhaseDeployment}
	results := v.phasesSkipped(cat, subset, "test reason")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Phase != PhaseDeployment {
		t.Errorf("Phase = %q, want %q", results[0].Phase, PhaseDeployment)
	}
}

func TestPhaseSkipped(t *testing.T) {
	v := New(WithVersion("1.0.0"))
	cat := loadEmbeddedCatalog(t)

	pr := v.phaseSkipped(cat, PhaseDeployment, "no cluster")
	if pr.Phase != PhaseDeployment {
		t.Errorf("Phase = %q, want %q", pr.Phase, PhaseDeployment)
	}
	if pr.Status != ctrf.StatusSkipped {
		t.Errorf("Status = %q, want %q", pr.Status, ctrf.StatusSkipped)
	}
	if pr.Report == nil {
		t.Fatal("Report should not be nil")
	}
	if pr.Report.ReportFormat != ctrf.ReportFormatCTRF {
		t.Errorf("ReportFormat = %q, want %q", pr.Report.ReportFormat, ctrf.ReportFormatCTRF)
	}
}

func TestValidatePhasesNoClusterAll(t *testing.T) {
	v := New(
		WithVersion("1.0.0"),
		WithNoCluster(true),
	)

	results, err := v.ValidatePhases(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatalf("ValidatePhases() failed: %v", err)
	}

	if len(results) != len(PhaseOrder) {
		t.Fatalf("expected %d results, got %d", len(PhaseOrder), len(results))
	}

	for _, pr := range results {
		if pr.Status != ctrf.StatusSkipped {
			t.Errorf("phase %q status = %q, want %q", pr.Phase, pr.Status, ctrf.StatusSkipped)
		}
	}
}

func TestValidatePhasesNoClusterSubset(t *testing.T) {
	v := New(
		WithVersion("1.0.0"),
		WithNoCluster(true),
	)

	results, err := v.ValidatePhases(context.Background(), []Phase{PhaseDeployment}, nil, nil)
	if err != nil {
		t.Fatalf("ValidatePhases() failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Phase != PhaseDeployment {
		t.Errorf("Phase = %q, want %q", results[0].Phase, PhaseDeployment)
	}
}

func TestValidatePhaseNoCluster(t *testing.T) {
	v := New(
		WithVersion("1.0.0"),
		WithNoCluster(true),
	)

	pr, err := v.ValidatePhase(context.Background(), PhaseDeployment, nil, nil)
	if err != nil {
		t.Fatalf("ValidatePhase() failed: %v", err)
	}

	if pr.Status != ctrf.StatusSkipped {
		t.Errorf("status = %q, want %q", pr.Status, ctrf.StatusSkipped)
	}
	if pr.Phase != PhaseDeployment {
		t.Errorf("phase = %q, want %q", pr.Phase, PhaseDeployment)
	}
}

func TestPhaseOrder(t *testing.T) {
	expected := []Phase{PhaseDeployment, PhasePerformance, PhaseConformance}
	if len(PhaseOrder) != len(expected) {
		t.Fatalf("PhaseOrder length = %d, want %d", len(PhaseOrder), len(expected))
	}
	for i, p := range PhaseOrder {
		if p != expected[i] {
			t.Errorf("PhaseOrder[%d] = %q, want %q", i, p, expected[i])
		}
	}
}
