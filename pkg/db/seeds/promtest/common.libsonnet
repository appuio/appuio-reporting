local formatLabels = function(labels)
  local lf = std.join(', ', std.map(function(l) '%s="%s"' % [ l, labels[l] ], std.objectFields(labels)));
  "{%s}" % [ lf ];

local series = function(name, labels, values) {
  series: name+formatLabels(labels),
  values: values,
};

// returns a test object with the given series and samples. Sample interval is 30s
// the evaluation time is set one hour in the future since all our queries operate on a 1h window
local test = function(name, series, query, samples) {
  name: name,
  interval: '30s',
  input_series: if std.isArray(series) then series else std.objectValues(series),
  promql_expr_test: [
    {
      expr: query,
      eval_time: '1h',
      exp_samples: if std.isArray(samples) then samples else [ samples ],
    },
  ],
};

{
  series: series,
  formatLabels: formatLabels,
  test: test,
}
