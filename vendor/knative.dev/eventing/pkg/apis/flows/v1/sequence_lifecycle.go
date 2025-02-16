/*
Copyright 2020 The Knative Authors

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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var sCondSet = apis.NewLivingConditionSet(SequenceConditionReady, SequenceConditionChannelsReady, SequenceConditionSubscriptionsReady, SequenceConditionAddressable,
	SequenceConditionOIDCIdentityCreated)

const (
	// SequenceConditionReady has status True when all subconditions below have been set to True.
	SequenceConditionReady = apis.ConditionReady

	// SequenceConditionChannelsReady has status True when all the channels created as part of
	// this sequence are ready.
	SequenceConditionChannelsReady apis.ConditionType = "ChannelsReady"

	// SequenceConditionSubscriptionsReady has status True when all the subscriptions created as part of
	// this sequence are ready.
	SequenceConditionSubscriptionsReady apis.ConditionType = "SubscriptionsReady"

	// SequenceConditionAddressable has status true when this Sequence meets
	// the Addressable contract and has a non-empty hostname.
	SequenceConditionAddressable apis.ConditionType = "Addressable"

	// SequenceConditionOIDCIdentityCreated has status True when the OIDCIdentity has been created.
	// This condition is only relevant if the OIDC feature is enabled.
	SequenceConditionOIDCIdentityCreated apis.ConditionType = "OIDCIdentityCreated"
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*Sequence) GetConditionSet() apis.ConditionSet {
	return sCondSet
}

// GetGroupVersionKind returns GroupVersionKind for InMemoryChannels
func (*Sequence) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Sequence")
}

// GetUntypedSpec returns the spec of the Sequence.
func (s *Sequence) GetUntypedSpec() interface{} {
	return s.Spec
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (ss *SequenceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return sCondSet.Manage(ss).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ss *SequenceStatus) IsReady() bool {
	return sCondSet.Manage(ss).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ss *SequenceStatus) InitializeConditions() {
	sCondSet.Manage(ss).InitializeConditions()
}

// PropagateSubscriptionStatuses sets the SubscriptionStatuses and SequenceConditionSubscriptionsReady based on
// the status of the incoming subscriptions.
func (ss *SequenceStatus) PropagateSubscriptionStatuses(subscriptions []*messagingv1.Subscription) {
	ss.SubscriptionStatuses = make([]SequenceSubscriptionStatus, len(subscriptions))
	allReady := true
	// If there are no subscriptions, treat that as a False case. Could go either way, but this seems right.
	if len(subscriptions) == 0 {
		allReady = false

	}
	for i, s := range subscriptions {
		ss.SubscriptionStatuses[i] = SequenceSubscriptionStatus{
			Subscription: corev1.ObjectReference{
				APIVersion: s.APIVersion,
				Kind:       s.Kind,
				Name:       s.Name,
				Namespace:  s.Namespace,
			},
		}

		if readyCondition := s.Status.GetCondition(messagingv1.SubscriptionConditionReady); readyCondition != nil {
			ss.SubscriptionStatuses[i].ReadyCondition = *readyCondition
			if !readyCondition.IsTrue() {
				allReady = false
			}
		} else {
			ss.SubscriptionStatuses[i].ReadyCondition = apis.Condition{
				Type:               apis.ConditionReady,
				Status:             corev1.ConditionUnknown,
				Reason:             "NoReady",
				Message:            "Subscription does not have Ready condition",
				LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now())},
			}
			allReady = false
		}

	}
	if allReady {
		sCondSet.Manage(ss).MarkTrue(SequenceConditionSubscriptionsReady)
	} else {
		ss.MarkSubscriptionsNotReady("SubscriptionsNotReady", "Subscriptions are not ready yet, or there are none")
	}
}

// PropagateChannelStatuses sets the ChannelStatuses and SequenceConditionChannelsReady based on the
// status of the incoming channels.
func (ss *SequenceStatus) PropagateChannelStatuses(channels []*eventingduckv1.Channelable) {
	ss.ChannelStatuses = make([]SequenceChannelStatus, len(channels))
	allReady := true
	// If there are no channels, treat that as a False case. Could go either way, but this seems right.
	if len(channels) == 0 {
		allReady = false

	}
	for i, c := range channels {
		// Mark the Sequence address as the Address of the first channel.
		if i == 0 {
			ss.setAddress(c.Status.Address)
		}

		ss.ChannelStatuses[i] = SequenceChannelStatus{
			Channel: corev1.ObjectReference{
				APIVersion: c.APIVersion,
				Kind:       c.Kind,
				Name:       c.Name,
				Namespace:  c.Namespace,
			},
		}

		if ready := c.Status.GetCondition(apis.ConditionReady); ready != nil {
			ss.ChannelStatuses[i].ReadyCondition = *ready
			if !ready.IsTrue() {
				allReady = false
			}
		} else {
			ss.ChannelStatuses[i].ReadyCondition = apis.Condition{
				Type:               apis.ConditionReady,
				Status:             corev1.ConditionUnknown,
				Reason:             "NoReady",
				Message:            "Channel does not have Ready condition",
				LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now())},
			}
			allReady = false
		}
	}
	if allReady {
		sCondSet.Manage(ss).MarkTrue(SequenceConditionChannelsReady)
	} else {
		ss.MarkChannelsNotReady("ChannelsNotReady", "Channels are not ready yet, or there are none")
	}
}

func (ss *SequenceStatus) MarkChannelsNotReady(reason, messageFormat string, messageA ...interface{}) {
	sCondSet.Manage(ss).MarkUnknown(SequenceConditionChannelsReady, reason, messageFormat, messageA...)
}

func (ss *SequenceStatus) MarkSubscriptionsNotReady(reason, messageFormat string, messageA ...interface{}) {
	sCondSet.Manage(ss).MarkUnknown(SequenceConditionSubscriptionsReady, reason, messageFormat, messageA...)
}

func (ss *SequenceStatus) MarkAddressableNotReady(reason, messageFormat string, messageA ...interface{}) {
	sCondSet.Manage(ss).MarkUnknown(SequenceConditionAddressable, reason, messageFormat, messageA...)
}

func (ss *SequenceStatus) setAddress(address *duckv1.Addressable) {
	if address == nil || address.URL == nil {
		ss.Address = duckv1.Addressable{}
		sCondSet.Manage(ss).MarkUnknown(SequenceConditionAddressable, "emptyAddress", "addressable is nil")
	} else {
		ss.Address = duckv1.Addressable{URL: address.URL}
		sCondSet.Manage(ss).MarkTrue(SequenceConditionAddressable)
	}
}

// MarkOIDCIdentityCreatedSucceeded marks the OIDCIdentityCreated condition as true.
func (ss *SequenceStatus) MarkOIDCIdentityCreatedSucceeded() {
	sCondSet.Manage(ss).MarkTrue(SequenceConditionOIDCIdentityCreated)
}

// MarkOIDCIdentityCreatedSucceededWithReason marks the OIDCIdentityCreated condition as true with the given reason.
func (ss *SequenceStatus) MarkOIDCIdentityCreatedSucceededWithReason(reason, messageFormat string, messageA ...interface{}) {
	sCondSet.Manage(ss).MarkTrueWithReason(SequenceConditionOIDCIdentityCreated, reason, messageFormat, messageA...)
}

// MarkOIDCIdentityCreatedFailed marks the OIDCIdentityCreated condition as false with the given reason.
func (ss *SequenceStatus) MarkOIDCIdentityCreatedFailed(reason, messageFormat string, messageA ...interface{}) {
	sCondSet.Manage(ss).MarkFalse(SequenceConditionOIDCIdentityCreated, reason, messageFormat, messageA...)
}

// MarkOIDCIdentityCreatedUnknown marks the OIDCIdentityCreated condition as unknown with the given reason.
func (ss *SequenceStatus) MarkOIDCIdentityCreatedUnknown(reason, messageFormat string, messageA ...interface{}) {
	sCondSet.Manage(ss).MarkUnknown(SequenceConditionOIDCIdentityCreated, reason, messageFormat, messageA...)
}
