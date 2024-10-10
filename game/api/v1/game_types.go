/*
Copyright 2024 costa92.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Replicas 字段定义了游戏的副本数量,默认为 1,最小为 1,最大为 3。Host为游戏服务的地址。

// GameSpec defines the desired state of Game
type GameSpec struct {
	//+kubebuilder:default:=1
	//+kubebuilder:validation:Maximum:=3
	//+kubebuilder:validation:Minimum:=1
	Replicas int32  `json:"replicas,omitempty"`
	Image    string `json:"image,omitempty"`
	Host     string `json:"host,omitempty"`
}

const (
	Init     = "Init"
	Running  = "Running"
	NotReady = "NotReady"
	Failed   = "Failed"
)

// GameStatus defines the observed state of Game
type GameStatus struct {
	// Phase 字段定义了游戏的当前状态,可以是 Init、Running、NotReady 或 Failed。
	Phase string `json:"phase,omitempty"`
	// Replicas 字段定义了游戏的副本数量。
	Replicas int32 `json:"replicas,omitempty"`
	// ReadyReplicas 字段定义了游戏的就绪副本数量。
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// LabelSelector 字段定义了游戏的标签选择器。用于HPA。
	LabelSelector string `json:"labelSelector,omitempty"`
}

// 添加了scale子资源，并添加子资源specpath，statuspath，selectorpath对应字段的定义。最后添加打印列。
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas,selectorpath=.status.labelSelector
//+kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=`.status.phase`,description="Game Phase"
//+kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.host`,description="Host address"
//+kubebuilder:printcolumn:name="DESIRED",type=integer,JSONPath=`.spec.replicas`,description="Desired Replicas"
//+kubebuilder:printcolumn:name="CURRENT",type=integer,JSONPath=`.status.replicas`,description="Current Replicas"
//+kubebuilder:printcolumn:name="READY",type=integer,JSONPath=`.status.readyReplicas`,description="Ready Replicas"
//+kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`,description="CreationTimestamp"

// Game is the Schema for the games API
type Game struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GameSpec   `json:"spec,omitempty"`
	Status GameStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GameList contains a list of Game
type GameList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Game `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Game{}, &GameList{})
}
