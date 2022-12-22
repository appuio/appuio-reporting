local c = import 'common.libsonnet';

local query = importstr '../appuio_cloud_memory.promql';
local subCPUQuery = importstr '../appuio_cloud_memory_sub_cpu.promql';
local subMemoryQuery = importstr '../appuio_cloud_memory_sub_memory.promql';

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

local subQueryTests = [
  c.test('sub CPU requests query sanity check',
         baseSeries,
         subCPUQuery,
         {
           labels: c.formatLabels(baseCalculatedLabels),
           value: 0,
         }),
  c.test('sub memory requests query sanity check',
         baseSeries,
         subMemoryQuery,
         {
           labels: c.formatLabels(baseCalculatedLabels),
           value: (minMemoryRequestMib - (1 / 1024 / 1024)) * 60,
         }),
];

{
  tests: subQueryTests + [
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
