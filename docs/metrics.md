# Metrics

The controller exports bounded Prometheus metrics for builds, build failures and duration, policy violations, artifact count and size, source download duration and bytes, and active artifacts. Labels are limited to result, reason, and input kind; resource names, namespaces, revisions, digests, URLs, and diagnostic messages are never labels.
