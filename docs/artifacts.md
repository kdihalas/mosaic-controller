# Artifacts

Generated archives contain `deploy/kustomization.yaml` and `deploy/resources.yaml`. `metadata/bundle.json` and `metadata/policy-report.json` default on; graph and provenance default off. The HTTP endpoint serves regular files only, rejects traversal, and never lists directories.

The controller verifies existing and newly archived artifacts. Missing or corrupt output is rebuilt. Retention is controlled by the standard Artifact SDK storage flags.
