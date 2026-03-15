# AI Cluster Runtime Demo

## Generating a Recipe

Recipe captures the exact constraints and component versions required for your target state. You can specify your environment explicitly:

```shell
aicr recipe \
  --service eks \
  --accelerator h100 \
  --intent training \
  --os ubuntu \
  --platform kubeflow \
  --output recipe.yaml
```

Or, you can point `aicr` at any existing GPU-accelerated Kubernetes cluster (EKS, GKE, AKS, or self-managed) to auto-discover that state:

```shell
aicr snapshot \
    --namespace aicr-validation \
    --node-selector nodeGroup=gpu-worker \
    --output snapshot.yaml

aicr recipe \
  --snapshot snapshot.yaml \
  --intent training \
  --node-selector nodeGroup=gpu-worker
```

## Bundling for Deployment

Once your `recipe.yaml` is generated, you convert it into deployable artifacts. For modern GitOps workflows, you can output directly to an OCI registry, or optionally include bundle cryptographic attestation using Sigstore:

```shell
aicr bundle \
  --recipe recipe.yaml \
  --deployer argocd \
  --output oci://ghcr.io/nvidia/bundle \
  --attest
```

\> Supply chain security is built into every layer. Every bundle includes checksums for all generated artifacts end-to-end, and the project itself ships with SLSA Level 3 provenance, SPDX SBOMs, and cosign image attestations.

## Validate the Cluster

Once deployed, AICR validates the cluster against the same recipe used to create the bundle. Validation runs across three phases:

| Phase | What it proves |
| :---- | :---- |
| `deployment` | Fabric is properly configured and operational |
| `performance` | Synthetic workloads meet expected throughput |
| `conformance` | Overall cluster conformance is verified |

These phases can be run individually or all at once. Validation reports are output in [Common Test Report Format (CTRF)](https://ctrf.io/) to enable programmatic post-processing.

```shell
aicr validate \
  --recipe recipe.yaml \
  --phase all \
  --output report.json
```

To verify CNCF AI Conformance, you can also add `--evidence-dir` to export evidence artifacts for [CNCF submission](https://github.com/NVIDIA/aicr/blob/main/docs/conformance/cncf/README.md).

## Contributing

The matrix of potential platform, GPU, OS, intent, and service configurations is bigger than any one team to manage. We built the framework, but we do need the community to help fill it in. If you've already validated and optimized a new configuration combination, run performance benchmarks, or written new deployment tests, please contribute to this project by opening a PR.

Also, do star the repo, try it on your cluster, and open an issue with your findings: [github.com/NVIDIA/aicr](https://github.com/NVIDIA/aicr).  
