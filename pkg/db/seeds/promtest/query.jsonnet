local c = import 'common.libsonnet';

local query = importstr '../appuio_cloud_memory.promql';

local commonLabels = {
  cluster_id: 'c-appuio-cloudscale-lpg-2',
  tenant_id: 'c-appuio-cloudscale-lpg-2',
};

{
  tests: [
    {
      interval: '30s',
      local runningUID = '35e3a8b1-b46d-496c-b2b7-1b52953bf904',
      local succeededUID = '2a7a6e32-0840-4ac3-bab4-52d7e16f4a0a',
      input_series: [
        c.series('kube_node_labels', commonLabels {
          label_appuio_io_node_class: 'flex',
          label_kubernetes_io_hostname: 'flex-x666',
          node: 'flex-x666',
        }, '1+0x10'),
        c.series('kube_namespace_labels', commonLabels {
          namespace: 'testproject',
          label_appuio_io_organization: 'cherry-pickers-inc',
        }, '1+0x10'),
        // Phases
        c.series('kube_pod_status_phase', commonLabels {
          namespace: 'testproject',
          phase: 'Succeeded',
          pod: 'succeeded-pod',
          uid: succeededUID,
        }, '1+0x10'),
        c.series('kube_pod_status_phase', commonLabels {
          namespace: 'testproject',
          phase: 'Running',
          pod: 'running-pod',
          uid: runningUID,
        }, '1+0x10'),
        // Requests
        c.series('kube_pod_container_resource_requests', commonLabels {
          namespace: 'testproject',
          pod: 'succeeded-pod',
          resource: 'memory',
          node: 'flex-x666',
          uid: succeededUID,
        }, '1+0x10'),
        c.series('kube_pod_container_resource_requests', commonLabels {
          namespace: 'testproject',
          pod: 'running-pod',
          resource: 'memory',
          node: 'flex-x666',
          uid: runningUID,
        }, '1+0x10'),
        c.series('kube_pod_container_resource_requests', commonLabels {
          namespace: 'testproject',
          pod: 'succeeded-pod',
          node: 'flex-x666',
          resource: 'cpu',
          uid: succeededUID,
        }, '0+0x10'),
        c.series('kube_pod_container_resource_requests', commonLabels {
          namespace: 'testproject',
          pod: 'running-pod',
          node: 'flex-x666',
          resource: 'cpu',
          uid: runningUID,
        }, '0+0x10'),
        // Real usage
        c.series('container_memory_working_set_bytes', commonLabels {
          image: 'busybox',
          namespace: 'testproject',
          pod: 'succeeded-pod',
          node: 'flex-x666',
          uid: succeededUID,
        }, '1+0x10'),
        c.series('container_memory_working_set_bytes', commonLabels {
          image: 'busybox',
          namespace: 'testproject',
          pod: 'running-pod',
          node: 'flex-x666',
          uid: runningUID,
        }, '1+0x10'),
      ],
      promql_expr_test: [
        {
          expr: query,
          eval_time: '1h',
          exp_samples: [
            {
              labels: c.formatLabels({
                category: 'c-appuio-cloudscale-lpg-2:testproject',
                cluster_id: 'c-appuio-cloudscale-lpg-2',
                label_appuio_io_node_class: 'flex',
                namespace: 'testproject',
                product: 'appuio_cloud_memory:c-appuio-cloudscale-lpg-2:cherry-pickers-inc:testproject:flex',
                tenant_id: 'cherry-pickers-inc',
              }),
              value: 128 * 10,
            },
          ],
        },
      ],
    },
  ],
}
