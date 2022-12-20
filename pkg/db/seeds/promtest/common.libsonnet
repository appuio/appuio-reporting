local formatLabels = function(labels)
  local lf = std.join(', ', std.map(function(l) '%s="%s"' % [ l, labels[l] ], std.objectFields(labels)));
  "{%s}" % [ lf ];

local series = function(name, labels, values) {
  series: name+formatLabels(labels),
  values: values,
};

{
  series: series,
  formatLabels: formatLabels,
}
