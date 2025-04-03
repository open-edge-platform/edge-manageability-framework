// SPDX-FileCopyrightText: 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"net/http"
	"time"

	baseorginfrahostcomv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/org.edge-orchestrator.intel.com/v1"
	baseprojectinfrahostcomv1 "github.com/open-edge-platform/orch-utils/tenancy-datamodel/build/apis/project.edge-orchestrator.intel.com/v1"
)

type orgs struct {
	Name   string                                `json:"name,omitempty" yaml:"name,omitempty"`
	Spec   *baseorginfrahostcomv1.OrgSpec        `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status *baseorginfrahostcomv1.OrgNexusStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type projects struct {
	Name   string                                        `json:"name,omitempty" yaml:"name,omitempty"`
	Spec   *baseprojectinfrahostcomv1.ProjectSpec        `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status *baseprojectinfrahostcomv1.ProjectNexusStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type metricsResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []any  `json:"result"`
	} `json:"data"`
}

type logsResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream struct {
				ServiceName string `json:"service_name"`
			} `json:"stream"`
		} `json:"result"`
		Values [][]any `json:"values"`
	} `json:"data"`
}

type AlertDefinitionsArray struct {
	AlertDefinitions []AlertDefinition `json:"alertDefinitions"`
}

type AlertDefinitionTemplate struct {
	Alert       string `json:"alert"`
	Annotations struct {
		AmDefinitionType string `json:"am_definition_type"`
		AmDuration       string `json:"am_duration"`
		AmDurationMax    string `json:"am_duration_max"`
		AmDurationMin    string `json:"am_duration_min"`
		AmEnabled        string `json:"am_enabled"`
		AmThreshold      string `json:"am_threshold"`
		AmThresholdMax   string `json:"am_threshold_max"`
		AmThresholdMin   string `json:"am_threshold_min"`
		AmThresholdUnit  string `json:"am_threshold_unit"`
		AmUUID           string `json:"am_uuid"`
		Description      string `json:"description"`
		DisplayName      string `json:"display_name"`
		Summary          string `json:"summary"`
	} `json:"annotations"`
	Expr   string `json:"expr"`
	For    string `json:"for"`
	Labels struct {
		AlertCategory string `json:"alert_category"`
		AlertContext  string `json:"alert_context"`
		Duration      string `json:"duration"`
		HostUUID      string `json:"host_uuid"`
		Threshold     string `json:"threshold"`
	} `json:"labels"`
}

type PatchDefinitionBody struct {
	Values PatchDefinitionValues `json:"values"`
}

type PatchDefinitionValues struct {
	Duration  string `json:"duration"`
	Threshold string `json:"threshold"`
	Enabled   string `json:"enabled"`
}

type AlertDefinition struct {
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	State    string                `json:"state"`
	Template string                `json:"template"`
	Values   PatchDefinitionValues `json:"values"`
	Version  int                   `json:"version"`
}

type Alerts struct {
	Alerts []struct {
		Labels struct {
			Alertname string `json:"alertname"`
			ProjectID string `json:"projectId"`
		} `json:"labels,omitempty"`
		Status struct {
			State string `json:"state"`
		} `json:"status"`
	} `json:"alerts"`
}

type AlertReceiversArray struct {
	Receivers []AlertReceiver `json:"receivers"`
}

type AlertReceiver struct {
	EmailConfig struct {
		From       string `json:"from"`
		MailServer string `json:"mailServer"`
		To         struct {
			Allowed []string `json:"allowed"`
			Enabled []string `json:"enabled"`
		} `json:"to"`
	} `json:"emailConfig"`
	ID      string `json:"id"`
	State   string `json:"state"`
	Version int    `json:"version"`
}

type PatchReceiverBody struct {
	EmailConfig struct {
		To struct {
			Enabled []string `json:"enabled"`
		} `json:"to"`
	} `json:"emailConfig"`
}

type MailList struct {
	Total    int `json:"total"`
	Messages []struct {
		ID        string `json:"ID"`
		MessageID string `json:"MessageID"`
		Read      bool   `json:"Read"`
		From      struct {
			Name    string `json:"Name"`
			Address string `json:"Address"`
		} `json:"From"`
		To []struct {
			Name    string `json:"Name"`
			Address string `json:"Address"`
		} `json:"To"`
		Cc          []any     `json:"Cc"`
		Bcc         []any     `json:"Bcc"`
		ReplyTo     []any     `json:"ReplyTo"`
		Subject     string    `json:"Subject"`
		Created     time.Time `json:"Created"`
		Tags        []any     `json:"Tags"`
		Size        int       `json:"Size"`
		Attachments int       `json:"Attachments"`
		Snippet     string    `json:"Snippet"`
	} `json:"messages"`
}

type APIClient struct {
	HTTPClient            *http.Client
	ServiceDomainWithPort string
	ProjectName           string
	Token                 string
}

type MaintenanceModeContext struct {
	RegionResourceID string
	SiteResourceID   string
	HostID           string
	ScheduleID       string
}

type GenericCreateResourceResponse struct {
	ResourceID string `json:"resourceId"`
}
