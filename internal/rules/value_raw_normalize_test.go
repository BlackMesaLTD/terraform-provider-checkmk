package rules

import "testing"

func TestValueRawSemanticEquals(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "identical",
			a:    "{'a': 'b'}",
			b:    "{'a': 'b'}",
			want: true,
		},
		{
			name: "whitespace after colon and comma",
			a:    "{'lower_levels':(2.0,1.0)}",
			b:    "{'lower_levels': (2.0, 1.0)}",
			want: true,
		},
		{
			name: "newlines and indentation in nested dict",
			a:    "{'m': [\n  {'k': 1},\n  {'k': 2}\n]}",
			b:    "{'m': [{'k': 1}, {'k': 2}]}",
			want: true,
		},
		{
			name: "meaningful spaces inside string are significant",
			a:    "{'q': 'sum(a b)'}",
			b:    "{'q': 'sum(ab)'}",
			want: false,
		},
		{
			name: "promql query whitespace preserved",
			a:    "{'promql_query': 'sum(rate(x[1m])) OR on() vector(0)'}",
			b:    "{'promql_query': 'sum(rate(x[1m])) OR on() vector(0)'}",
			want: true,
		},
		{
			name: "different value",
			a:    "{'a': 1}",
			b:    "{'a': 2}",
			want: false,
		},
		{
			name: "different key",
			a:    "{'a': 1}",
			b:    "{'b': 1}",
			want: false,
		},
		{
			name: "escaped quote inside string handled",
			a:    `{'desc': 'it\'s fine'}`,
			b:    `{'desc': 'it\'s fine'}`,
			want: true,
		},
		{
			name: "realistic prometheus rule differs only by structural whitespace",
			a:    "{'connection': 'prom.local', 'verify-cert': False, 'protocol': 'https', 'exporter': [('node_exporter', {'entities': ['df','mem']})], 'promql_checks': [{'service_description': 'X', 'metric_components': [{'metric_label': 'running', 'promql_query': 'sum(kube_pod_status_phase{phase=`Running`}) OR on() vector(0)', 'levels': {'lower_levels':(2.0,1.0)}}]}]}",
			b:    "{'connection': 'prom.local', 'verify-cert': False, 'protocol': 'https', 'exporter': [('node_exporter', {'entities': ['df', 'mem']})], 'promql_checks': [{'service_description': 'X', 'metric_components': [{'metric_label': 'running', 'promql_query': 'sum(kube_pod_status_phase{phase=`Running`}) OR on() vector(0)', 'levels': {'lower_levels': (2.0, 1.0)}}]}]}",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := valueRawSemanticEquals(tt.a, tt.b); got != tt.want {
				t.Errorf("valueRawSemanticEquals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizePythonLiteralPreservesStringContent(t *testing.T) {
	in := "{'q': 'a   b\tc'}"
	got := normalizePythonLiteral(in)
	want := "{'q':'a   b\tc'}"
	if got != want {
		t.Errorf("normalizePythonLiteral() = %q, want %q", got, want)
	}
}
