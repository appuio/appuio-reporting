local c = import 'common.libsonnet';

local query = '
# Sum values over one hour.
sum_over_time(
  # Average over a one-minute time frame.
  # NOTE: This is a sliding window. Results vary based on the queries execution time.
  avg_over_time(
    # Add the final product label by joining the base product with the cluster ID, the tenant and the namespace.
    label_join(
      # Add the category label by joining the cluster ID and the namespace.
      label_join(
        # Add the base product identifier.
        label_replace(
          clamp_min(
            # Get the maximum of requested and used memory.
            # TODO Is there a better way to get the maximum of two vectors?
            (
              (
                # Select used memory if higher.
                (
                  sum by(cluster_id, namespace, label_appuio_io_node_class) (container_memory_working_set_bytes{image!=""}
                    * on(cluster_id, node) group_left(label_appuio_io_node_class) (min by(cluster_id, node, label_appuio_io_node_class) (kube_node_labels{label_appuio_io_node_class!=""} or on(cluster_id, node) kube_node_labels{label_appuio_io_node_class=""})))
                  # IMPORTANT: one clause must use equal. If used grater and lesser than, equal values will be dropped.
                  >=
                  sum by(cluster_id, namespace, label_appuio_io_node_class) (kube_pod_container_resource_requests{resource="memory"}
                    * on(uid, cluster_id, pod, namespace) group_left kube_pod_status_phase{phase="Running"}
                    * on(cluster_id, node) group_left(label_appuio_io_node_class) (min by(cluster_id, node, label_appuio_io_node_class) (kube_node_labels{label_appuio_io_node_class!=""} or on(cluster_id, node) kube_node_labels{label_appuio_io_node_class=""})))
                )
                or
                # Select reserved memory if higher.
                (
                  # IMPORTANT: The desired time series must always be first.
                  sum by(cluster_id, namespace, label_appuio_io_node_class) (kube_pod_container_resource_requests{resource="memory"}
                    * on(uid, cluster_id, pod, namespace) group_left kube_pod_status_phase{phase="Running"}
                    * on(cluster_id, node) group_left(label_appuio_io_node_class) (min by(cluster_id, node, label_appuio_io_node_class) (kube_node_labels{label_appuio_io_node_class!=""} or on(cluster_id, node) kube_node_labels{label_appuio_io_node_class=""})))
                  >
                  sum by(cluster_id, namespace, label_appuio_io_node_class) (container_memory_working_set_bytes{image!=""}
                    * on(cluster_id, node) group_left(label_appuio_io_node_class) (min by(cluster_id, node, label_appuio_io_node_class) (kube_node_labels{label_appuio_io_node_class!=""} or on(cluster_id, node) kube_node_labels{label_appuio_io_node_class=""})))
                )
              )
              # Add CPU requests in violation to the ratio provided by the platform.
              + clamp_min(
                  # Convert CPU request to their memory equivalent.
                  sum by(cluster_id, namespace, label_appuio_io_node_class) (
                    kube_pod_container_resource_requests{resource="cpu"} * on(uid, cluster_id, pod, namespace) group_left kube_pod_status_phase{phase="Running"}
                      * on(cluster_id, node) group_left(label_appuio_io_node_class) (min by(cluster_id, node, label_appuio_io_node_class) (kube_node_labels{label_appuio_io_node_class!=""} or on(cluster_id, node) kube_node_labels{label_appuio_io_node_class=""}))
                    # Build that ratio from static values
                    * on(cluster_id) group_left()(
                      # Build a time series of ratio for Cloudscale LPG 2 (4096 MiB/core)
                      label_replace(vector(4294967296), "cluster_id", "c-appuio-cloudscale-lpg-2", "", "")
                      # Build a time series of ratio for Exoscale GVA-2 0 (5086 MiB/core)
                      or label_replace(vector(5333057536), "cluster_id", "c-appuio-exoscale-ch-gva-2-0", "", "")
                    )
                  )
                  # Subtract memory request
                  - sum by(cluster_id, namespace, label_appuio_io_node_class) (kube_pod_container_resource_requests{resource="memory"} * on(uid, cluster_id, pod, namespace) group_left kube_pod_status_phase{phase="Running"}
                    * on(cluster_id, node) group_left(label_appuio_io_node_class) (min by(cluster_id, node, label_appuio_io_node_class) (kube_node_labels{label_appuio_io_node_class!=""} or on(cluster_id, node) kube_node_labels{label_appuio_io_node_class=""}))
              # Only values above zero are in violation.
              ), 0)
            )
            *
            # Join namespace label `label_appuio_io_organization` as `tenant_id`.
            on(cluster_id, namespace)
            group_left(tenant_id)
            (
              bottomk(1,
                min by (cluster_id, namespace, tenant_id) (
                  label_replace(
                    kube_namespace_labels{label_appuio_io_organization=~".+"},
                    "tenant_id",
                    "$1",
                    "label_appuio_io_organization", "(.*)"
                  )
                )
              ) by(cluster_id, namespace)
            ),
            # At least return 128MiB
            128 * 1024 * 1024
          ),
          "product",
          "appuio_cloud_memory",
          "product",
          ".*"
        ),
        "category",
        ":",
        "cluster_id",
        "namespace"
      ),
      "product",
      ":",
      "product",
      "cluster_id",
      "tenant_id",
      "namespace",
      "label_appuio_io_node_class"
    )[45s:15s]
  )[59m:1m]
)
# Convert to MiB
/ 1024 / 1024
';

local commonLabels = {
  cluster_id: 'c-appuio-cloudscale-lpg-2',
  tenant_id: 'c-appuio-cloudscale-lpg-2',
};

// One running pod, minimal (=1 byte) memory request and usage, no CPU request
// 10 samples
local baseSeries = {
  flexNodeLabel: c.series('kube_node_labels', commonLabels {
    label_appuio_io_node_class: 'flex',
    label_kubernetes_io_hostname: 'flex-x666',
    node: 'flex-x666',
  }, '1x120'),
  testprojectNamespaceOrgLabel: c.series('kube_namespace_labels', commonLabels {
    namespace: 'testproject',
    label_appuio_io_organization: 'cherry-pickers-inc',
  }, '1x120'),

  local podLbls = commonLabels {
    namespace: 'testproject',
    pod: 'running-pod',
    uid: '35e3a8b1-b46d-496c-b2b7-1b52953bf904',
  },
  // Phases
  runningPodPhase: c.series('kube_pod_status_phase', podLbls {
    phase: 'Running',
  }, '1x120'),
  // Requests
  runningPodMemoryRequests: c.series('kube_pod_container_resource_requests', podLbls {
    resource: 'memory',
    node: 'flex-x666',
  }, '1x120'),
  runningPodCPURequests: c.series('kube_pod_container_resource_requests', podLbls {
    resource: 'cpu',
    node: 'flex-x666',
  }, '0x120'),
  // Real usage
  runningPodMemoryUsage: c.series('container_memory_working_set_bytes', podLbls {
    image: 'busybox',
    node: 'flex-x666',
  }, '1x120'),
};

local baseCalculatedLabels = {
  category: 'c-appuio-cloudscale-lpg-2:testproject',
  cluster_id: 'c-appuio-cloudscale-lpg-2',
  label_appuio_io_node_class: 'flex',
  namespace: 'testproject',
  product: 'appuio_cloud_memory:c-appuio-cloudscale-lpg-2:cherry-pickers-inc:testproject:flex',
  tenant_id: 'cherry-pickers-inc',
};

// Constants from the query
local minMemoryRequestMib = 128;
local cloudscaleFairUseRatio = 4294967296;


{
  tests: [
    c.test('minimal pod',
           baseSeries,
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             value: minMemoryRequestMib * 60,
           }),
    c.test('pod with higher memory usage',
           baseSeries {
             runningPodMemoryUsage+: {
               values: '%sx120' % (500 * 1024 * 1024),
             },
           },
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             value: 500 * 60,
           }),
    c.test('pod with higher memory requests',
           baseSeries {
             runningPodMemoryRequests+: {
               values: '%sx120' % (500 * 1024 * 1024),
             },
           },
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             value: 500 * 60,
           }),
    c.test('pod with CPU requests violating fair use',
           baseSeries {
             runningPodCPURequests+: {
               values: '1x120',
             },
           },
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             // See per cluster fair use ratio in query
             //  value: 2.048E+04,
             value: (cloudscaleFairUseRatio / 1024 / 1024) * 60,
           }),
    c.test('non-running pods are not counted',
           baseSeries {
             local lbls = commonLabels {
               namespace: 'testproject',
               pod: 'succeeded-pod',
               uid: '2a7a6e32-0840-4ac3-bab4-52d7e16f4a0a',
             },
             succeededPodPhase: c.series('kube_pod_status_phase', lbls {
               phase: 'Succeeded',
             }, '1x120'),
             succeededPodMemoryRequests: c.series('kube_pod_container_resource_requests', lbls {
               resource: 'memory',
               node: 'flex-x666',
             }, '1x120'),
             succeededPodCPURequests: c.series('kube_pod_container_resource_requests', lbls {
               node: 'flex-x666',
               resource: 'cpu',
             }, '1x120'),
           },
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             value: minMemoryRequestMib * 60,
           }),
    c.test('unrelated kube_node_labels changes do not throw errors - there is an overlap since series go stale only after a few missed scrapes',
           baseSeries {
             flexNodeLabelUpdated: self.flexNodeLabel {
               _labels+:: {
                 label_csi_driver_id: '18539CC3-0B6C-4E72-82BD-90A9BEF7D807',
               },
               values: '_x30 1x30 _x60',
             },
           },
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             value: minMemoryRequestMib * 60,
           }),
    c.test('node class adds do not throw errors - there is an overlap since series go stale only after a few missed scrapes',
           baseSeries {
             flexNodeLabel+: {
               _labels+:: {
                 label_appuio_io_node_class:: null,
               },
               values: '1x60',
             },
             flexNodeLabelUpdated: super.flexNodeLabel {
               values: '_x30 1x90',
             },
           },
           query,
           [
             // I'm not sure why this is 61min * minMemoryRequestMib. Other queries always result in 60min
             // TODO investigate where the extra min comes from
             {
               labels: c.formatLabels(baseCalculatedLabels),
               value: minMemoryRequestMib * 46,
             },
             {
               labels: c.formatLabels(baseCalculatedLabels {
                 label_appuio_io_node_class:: null,
                 product: 'appuio_cloud_memory:c-appuio-cloudscale-lpg-2:cherry-pickers-inc:testproject:',
               }),
               value: minMemoryRequestMib * 15,
             },
           ]),

    c.test('unrelated kube_namespace_labels changes do not throw errors - there is an overlap since series go stale only after a few missed scrapes',
           baseSeries {
             testprojectNamespaceOrgLabelUpdated: self.testprojectNamespaceOrgLabel {
               _labels+:: {
                 custom_appuio_io_myid: '672004be-a86b-44e0-b446-1255a1f8b340',
               },
               values: '_x30 1x30 _x60',
             },
           },
           query,
           {
             labels: c.formatLabels(baseCalculatedLabels),
             value: minMemoryRequestMib * 60,
           }),

    c.test('organization changes do not throw many-to-many errors - there is an overlap since series go stale only after a few missed scrapes',
           baseSeries {
             testprojectNamespaceOrgLabel+: {
               // We cheat here and use an impossible value.
               // Since we use min() and bottomk() in the query this priotizes this series less than the other.
               // It's ugly but it prevents flaky tests since otherwise one of the series gets picked randomly.
               // Does not influence the result. The result is floored to a minimum of 128MiB.
               values: '2x120',
             },
             testprojectNamespaceOrgLabelUpdated: self.testprojectNamespaceOrgLabel {
               _labels+:: {
                 label_appuio_io_organization: 'carrot-pickers-inc',
               },
               values: '_x60 1x60',
             },
           },
           query,
           [
             // I'm not sure why this is 61min * minMemoryRequestMib. Other queries always result in 60min
             // TODO investigate where the extra min comes from
             {
               labels: c.formatLabels(baseCalculatedLabels),
               value: minMemoryRequestMib * 30,
             },
             {
               labels: c.formatLabels(baseCalculatedLabels {
                 tenant_id: 'carrot-pickers-inc',
                 product: 'appuio_cloud_memory:c-appuio-cloudscale-lpg-2:carrot-pickers-inc:testproject:flex',
               }),
               value: minMemoryRequestMib * 31,
             },
           ]),

  ],
}
