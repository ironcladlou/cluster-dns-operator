package controller

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	configv1 "github.com/openshift/api/config/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestComputeOperatorStatusConditions(t *testing.T) {
	type conditions struct {
		degraded, progressing, available bool
	}
	type versions struct {
		operator, coreDNSOperand, openshiftCLIOperand string
	}

	testCases := []struct {
		description      string
		noNamespace      bool
		dnses            dnsStatusConditionsCounts
		reportedVersions versions
		oldVersions      versions
		curVersions      versions
		expected         conditions
	}{
		{
			description: "no operand namespace or dnses available",
			noNamespace: true,
			expected:    conditions{available: false, progressing: true, degraded: true},
		},
		{
			description: "0/0 dns resources available",
			dnses:       dnsStatusConditionsCounts{available: 0, progressing: 0, degraded: 0, total: 0},
			expected:    conditions{available: false, progressing: true, degraded: true},
		},
		{
			description: "1/2 dns resources available",
			dnses:       dnsStatusConditionsCounts{available: 1, progressing: 1, degraded: 1, total: 2},
			expected:    conditions{available: true, progressing: true, degraded: true},
		},
		{
			description: "2/2 dns resources available",
			dnses:       dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			expected:    conditions{available: true, progressing: false, degraded: false},
		},
		{
			description:      "versions match",
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			reportedVersions: versions{"v1", "dns-v1", "cli-v1"},
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v1", "dns-v1", "cli-v1"},
			expected:         conditions{available: true, progressing: false, degraded: false},
		},
		{
			description:      "operator upgrade in progress",
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 2, degraded: 2, total: 2},
			reportedVersions: versions{"v1", "dns-v1", "cli-v1"},
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v2", "dns-v1", "cli-v1"},
			expected:         conditions{available: true, progressing: true, degraded: true},
		},
		{
			description:      "coredns upgrade in progress",
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 2, degraded: 2, total: 2},
			reportedVersions: versions{"v1", "dns-v1", "cli-v1"},
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v1", "dns-v2", "cli-v1"},
			expected:         conditions{available: true, progressing: true, degraded: true},
		},
		{
			description:      "openshift-cli upgrade in progress",
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 2, degraded: 2, total: 2},
			reportedVersions: versions{"v1", "dns-v1", "cli-v1"},
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v1", "dns-v1", "cli-v2"},
			expected:         conditions{available: true, progressing: true, degraded: true},
		},
		{
			description:      "operator, coredns and openshift-cli upgrade in progress",
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 2, degraded: 2, total: 2},
			reportedVersions: versions{"v1", "dns-v1", "cli-v1"},
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v2", "dns-v2", "cli-v2"},
			expected:         conditions{available: true, progressing: true, degraded: true},
		},
		{
			description:      "operator upgrade done",
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			reportedVersions: versions{"v2", "dns-v1", "cli-v1"},
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v2", "dns-v1", "cli-v1"},
			expected:         conditions{available: true, progressing: false, degraded: false},
		},
		{
			description:      "coredns upgrade done",
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			reportedVersions: versions{"v1", "dns-v2", "cli-v1"},
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v1", "dns-v2", "cli-v1"},
			expected:         conditions{available: true, progressing: false, degraded: false},
		},
		{
			description:      "openshift-cli upgrade done",
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			reportedVersions: versions{"v1", "dns-v1", "cli-v2"},
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v1", "dns-v1", "cli-v2"},
			expected:         conditions{available: true, progressing: false, degraded: false},
		},
		{
			description:      "operator, coredns and openshift-cli upgrade done",
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			reportedVersions: versions{"v2", "dns-v2", "cli-v2"},
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v2", "dns-v2", "cli-v2"},
			expected:         conditions{available: true, progressing: false, degraded: false},
		},
		{
			description:      "operator upgrade in progress, coredns upgrade done",
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 2, degraded: 2, total: 2},
			reportedVersions: versions{"v1", "dns-v1", "cli-v2"},
			oldVersions:      versions{"v1", "dns-v2", "cli-v2"},
			curVersions:      versions{"v2", "dns-v2", "cli-v2"},
			expected:         conditions{available: true, progressing: true, degraded: true},
		},
	}

	for _, tc := range testCases {
		var namespace *corev1.Namespace
		if !tc.noNamespace {
			namespace = &corev1.Namespace{}
		}

		oldVersions := []configv1.OperandVersion{
			{
				Name:    OperatorVersionName,
				Version: tc.oldVersions.operator,
			},
			{
				Name:    CoreDNSVersionName,
				Version: tc.oldVersions.coreDNSOperand,
			},
		}
		reportedVersions := []configv1.OperandVersion{
			{
				Name:    OperatorVersionName,
				Version: tc.reportedVersions.operator,
			},
			{
				Name:    CoreDNSVersionName,
				Version: tc.reportedVersions.coreDNSOperand,
			},
		}
		r := &reconciler{
			Config: Config{
				OperatorReleaseVersion: tc.curVersions.operator,
				CoreDNSImage:           tc.curVersions.coreDNSOperand,
			},
		}

		expectedConditions := []configv1.ClusterOperatorStatusCondition{
			{
				Type:   configv1.OperatorDegraded,
				Status: configv1.ConditionFalse,
			},
			{
				Type:   configv1.OperatorProgressing,
				Status: configv1.ConditionFalse,
			},
			{
				Type:   configv1.OperatorAvailable,
				Status: configv1.ConditionFalse,
			},
		}
		if tc.expected.degraded {
			expectedConditions[0].Status = configv1.ConditionTrue
		}
		if tc.expected.progressing {
			expectedConditions[1].Status = configv1.ConditionTrue
		}
		if tc.expected.available {
			expectedConditions[2].Status = configv1.ConditionTrue
		}

		conditions := r.computeOperatorStatusConditions([]configv1.ClusterOperatorStatusCondition{}, namespace,
			tc.dnses, oldVersions, reportedVersions)
		conditionsCmpOpts := []cmp.Option{
			cmpopts.IgnoreFields(configv1.ClusterOperatorStatusCondition{}, "LastTransitionTime", "Reason", "Message"),
			cmpopts.EquateEmpty(),
			cmpopts.SortSlices(func(a, b configv1.ClusterOperatorStatusCondition) bool { return a.Type < b.Type }),
		}
		if !cmp.Equal(conditions, expectedConditions, conditionsCmpOpts...) {
			t.Errorf("%q: expected %#v, got %#v", tc.description, expectedConditions, conditions)
		}
	}
}

func TestOperatorStatusesEqual(t *testing.T) {
	testCases := []struct {
		description string
		expected    bool
		a, b        configv1.ClusterOperatorStatus
	}{
		{
			description: "zero-valued ClusterOperatorStatus should be equal",
			expected:    true,
		},
		{
			description: "nil and non-nil slices are equal",
			expected:    true,
			a: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{},
			},
		},
		{
			description: "empty slices should be equal",
			expected:    true,
			a: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{},
			},
			b: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{},
			},
		},
		{
			description: "check no change in versions",
			expected:    true,
			a: configv1.ClusterOperatorStatus{
				Versions: []configv1.OperandVersion{
					{
						Name:    "operator",
						Version: "v1",
					},
					{
						Name:    "coredns",
						Version: "v2",
					},
				},
			},
			b: configv1.ClusterOperatorStatus{
				Versions: []configv1.OperandVersion{
					{
						Name:    "operator",
						Version: "v1",
					},
					{
						Name:    "coredns",
						Version: "v2",
					},
				},
			},
		},
		{
			description: "condition LastTransitionTime should not be ignored",
			expected:    false,
			a: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{
					{
						Type:               configv1.OperatorAvailable,
						Status:             configv1.ConditionTrue,
						LastTransitionTime: metav1.Unix(0, 0),
					},
				},
			},
			b: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{
					{
						Type:               configv1.OperatorAvailable,
						Status:             configv1.ConditionTrue,
						LastTransitionTime: metav1.Unix(1, 0),
					},
				},
			},
		},
		{
			description: "order of versions should not matter",
			expected:    true,
			a: configv1.ClusterOperatorStatus{
				Versions: []configv1.OperandVersion{
					{
						Name:    "operator",
						Version: "v1",
					},
					{
						Name:    "coredns",
						Version: "v2",
					},
				},
			},
			b: configv1.ClusterOperatorStatus{
				Versions: []configv1.OperandVersion{
					{
						Name:    "coredns",
						Version: "v2",
					},
					{
						Name:    "operator",
						Version: "v1",
					},
				},
			},
		},
		{
			description: "check missing related objects",
			expected:    false,
			a: configv1.ClusterOperatorStatus{
				RelatedObjects: []configv1.ObjectReference{
					{
						Name: "openshift-dns",
					},
					{
						Name: "default",
					},
				},
			},
			b: configv1.ClusterOperatorStatus{
				RelatedObjects: []configv1.ObjectReference{
					{
						Name: "default",
					},
				},
			},
		},
		{
			description: "check extra related objects",
			expected:    false,
			a: configv1.ClusterOperatorStatus{
				RelatedObjects: []configv1.ObjectReference{
					{
						Name: "default",
					},
				},
			},
			b: configv1.ClusterOperatorStatus{
				RelatedObjects: []configv1.ObjectReference{
					{
						Name: "openshift-dns",
					},
					{
						Name: "default",
					},
				},
			},
		},
		{
			description: "check condition reason differs",
			expected:    false,
			a: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{
					{
						Type:   configv1.OperatorAvailable,
						Status: configv1.ConditionFalse,
						Reason: "foo",
					},
				},
			},
			b: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{
					{
						Type:   configv1.OperatorAvailable,
						Status: configv1.ConditionFalse,
						Reason: "bar",
					},
				},
			},
		},
		{
			description: "check condition message differs",
			expected:    false,
			a: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{
					{
						Type:    configv1.OperatorAvailable,
						Status:  configv1.ConditionFalse,
						Message: "foo",
					},
				},
			},
			b: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{
					{
						Type:    configv1.OperatorAvailable,
						Status:  configv1.ConditionFalse,
						Message: "bar",
					},
				},
			},
		},
		{
			description: "check duplicate with single condition",
			expected:    false,
			a: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{
					{
						Type:    configv1.OperatorAvailable,
						Message: "foo",
					},
				},
			},
			b: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{
					{
						Type:    configv1.OperatorAvailable,
						Message: "foo",
					},
					{
						Type:    configv1.OperatorAvailable,
						Message: "foo",
					},
				},
			},
		},
		{
			description: "check duplicate with multiple conditions",
			expected:    false,
			a: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{
					{
						Type: configv1.OperatorAvailable,
					},
					{
						Type: configv1.OperatorProgressing,
					},
					{
						Type: configv1.OperatorAvailable,
					},
				},
			},
			b: configv1.ClusterOperatorStatus{
				Conditions: []configv1.ClusterOperatorStatusCondition{
					{
						Type: configv1.OperatorProgressing,
					},
					{
						Type: configv1.OperatorAvailable,
					},
					{
						Type: configv1.OperatorProgressing,
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		if actual := operatorStatusesEqual(tc.a, tc.b); actual != tc.expected {
			t.Fatalf("%q: expected %v, got %v", tc.description, tc.expected, actual)
		}
	}
}

func TestComputeOperatorStatusVersions(t *testing.T) {
	type versions struct {
		operator, coreDNSOperand, openshiftCLIOperand string
	}

	testCases := []struct {
		description      string
		oldVersions      versions
		curVersions      versions
		dnses            dnsStatusConditionsCounts
		expectedVersions versions
	}{
		{
			description:      "initialize versions, DNSes available",
			oldVersions:      versions{UnknownVersionValue, UnknownVersionValue, UnknownVersionValue},
			curVersions:      versions{"v1", "dns-v1", "cli-v1"},
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			expectedVersions: versions{"v1", "dns-v1", "cli-v1"},
		},
		{
			description:      "initialize versions, DNSes not all available",
			oldVersions:      versions{UnknownVersionValue, UnknownVersionValue, UnknownVersionValue},
			curVersions:      versions{"v1", "dns-v1", "cli-v1"},
			dnses:            dnsStatusConditionsCounts{available: 0, progressing: 2, degraded: 2, total: 2},
			expectedVersions: versions{UnknownVersionValue, UnknownVersionValue, UnknownVersionValue},
		},
		{
			description:      "update with no change",
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v1", "dns-v1", "cli-v1"},
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			expectedVersions: versions{"v1", "dns-v1", "cli-v1"},
		},
		{
			description:      "update operator version, DNSes not all available",
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v2", "dns-v1", "cli-v1"},
			dnses:            dnsStatusConditionsCounts{available: 0, progressing: 2, degraded: 2, total: 2},
			expectedVersions: versions{"v1", "dns-v1", "cli-v1"},
		},
		{
			description:      "update operator version, DNSes available",
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v2", "dns-v1", "cli-v1"},
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			expectedVersions: versions{"v2", "dns-v1", "cli-v1"},
		},
		{
			description:      "update coredns image, DNSes not all available",
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v1", "dns-v2", "cli-v1"},
			dnses:            dnsStatusConditionsCounts{available: 0, progressing: 2, degraded: 2, total: 2},
			expectedVersions: versions{"v1", "dns-v1", "cli-v1"},
		},
		{
			description:      "update coredns image, DNSes available",
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v1", "dns-v2", "cli-v1"},
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			expectedVersions: versions{"v1", "dns-v2", "cli-v1"},
		},
		{
			description:      "update openshift-cli image, DNSes not all available",
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v1", "dns-v1", "cli-v2"},
			dnses:            dnsStatusConditionsCounts{available: 0, progressing: 2, degraded: 2, total: 2},
			expectedVersions: versions{"v1", "dns-v1", "cli-v1"},
		},
		{
			description:      "update openshift-cli image, DNSes available",
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v1", "dns-v1", "cli-v2"},
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			expectedVersions: versions{"v1", "dns-v1", "cli-v2"},
		},
		{
			description:      "update operator, coredns and openshift-cli image, DNSes not all available",
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v2", "dns-v2", "cli-v2"},
			dnses:            dnsStatusConditionsCounts{available: 0, progressing: 2, degraded: 2, total: 2},
			expectedVersions: versions{"v1", "dns-v1", "cli-v1"},
		},
		{
			description:      "update operator, coredns and openshift-cli image, DNSes available",
			oldVersions:      versions{"v1", "dns-v1", "cli-v1"},
			curVersions:      versions{"v2", "dns-v2", "cli-v2"},
			dnses:            dnsStatusConditionsCounts{available: 2, progressing: 0, degraded: 0, total: 2},
			expectedVersions: versions{"v2", "dns-v2", "cli-v2"},
		},
	}

	for _, tc := range testCases {
		var (
			oldVersions      []configv1.OperandVersion
			expectedVersions []configv1.OperandVersion
		)

		oldVersions = []configv1.OperandVersion{
			{
				Name:    OperatorVersionName,
				Version: tc.oldVersions.operator,
			},
			{
				Name:    CoreDNSVersionName,
				Version: tc.oldVersions.coreDNSOperand,
			},
		}
		expectedVersions = []configv1.OperandVersion{
			{
				Name:    OperatorVersionName,
				Version: tc.expectedVersions.operator,
			},
			{
				Name:    CoreDNSVersionName,
				Version: tc.expectedVersions.coreDNSOperand,
			},
		}

		r := &reconciler{
			Config: Config{
				OperatorReleaseVersion: tc.curVersions.operator,
				CoreDNSImage:           tc.curVersions.coreDNSOperand,
			},
		}
		versions := r.computeOperatorStatusVersions(oldVersions, tc.dnses)
		versionsCmpOpts := []cmp.Option{
			cmpopts.EquateEmpty(),
			cmpopts.SortSlices(func(a, b configv1.OperandVersion) bool { return a.Name < b.Name }),
		}
		if !cmp.Equal(versions, expectedVersions, versionsCmpOpts...) {
			t.Errorf("%q: expected %v, got %v", tc.description, expectedVersions, versions)
		}
	}
}
