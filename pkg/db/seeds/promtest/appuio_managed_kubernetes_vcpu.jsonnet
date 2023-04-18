local c = import 'common.libsonnet';

local query = importstr '../appuio_managed_kubernetes_vcpu.promql';

local commonLabels = {
  cluster_id: 'c-managed-kubernetes',
  tenant_id: 't-managed-kubernetes',
  vshn_service_level: 'standard',
};

local baseSeries = {
  appNodeCPUInfoLabel0: c.series('node_cpu_info', commonLabels {
    instance: 'app-test',
    cpu: '0',
  }, '1x120'),
  appNodeCPUInfoLabel1: c.series('node_cpu_info', commonLabels {
    instance: 'app-test',
    cpu: '1',
  }, '1x120'),
  appNodeCPUInfoLabel2: c.series('node_cpu_info', commonLabels {
    instance: 'app-test2',
    cpu: '0',
  }, '1x120'),
};

local baseCalculatedLabels = commonLabels {
  class: super.vshn_service_level,
  category: super.tenant_id + ':' + super.cluster_id,
};

{
  tests: [
    c.test(
      'two app CPUs and one storage CPU',
      baseSeries,
      query,
      [
        {
          labels: c.formatLabels(baseCalculatedLabels {
            product: 'appuio_managed_kubernetes_vcpu:c-managed-kubernetes:t-managed-kubernetes:standard',
          }),
          value: 3,
        },
      ]
    ),

  ],
}
