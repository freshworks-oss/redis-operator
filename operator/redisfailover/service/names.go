package service

import (
	"fmt"
	"strconv"
	"strings"

	redisfailoverv1 "github.com/freshworks/redis-operator/api/redisfailover/v1"
	corev1 "k8s.io/api/core/v1"
)

// GetRedisShutdownConfigMapName returns the name for redis configmap
func GetRedisShutdownConfigMapName(rf *redisfailoverv1.RedisFailover) string {
	if rf.Spec.Redis.ShutdownConfigMap != "" {
		return rf.Spec.Redis.ShutdownConfigMap
	}
	return GetRedisShutdownName(rf)
}

// GetRedisName returns the name for redis resources
func GetRedisName(rf *redisfailoverv1.RedisFailover) string {
	return generateName(redisName, rf.Name)
}

// GetRedisShutdownName returns the name for redis resources
func GetRedisShutdownName(rf *redisfailoverv1.RedisFailover) string {
	return generateName(redisShutdownName, rf.Name)
}

// GetRedisReadinessName returns the name for redis resources
func GetRedisReadinessName(rf *redisfailoverv1.RedisFailover) string {
	return generateName(redisReadinessName, rf.Name)
}

// GetSentinelName returns the name for sentinel resources
func GetSentinelName(rf *redisfailoverv1.RedisFailover) string {
	return generateName(sentinelName, rf.Name)
}

func GetRedisMasterName(rf *redisfailoverv1.RedisFailover) string {
	return generateName(redisMasterName, rf.Name)
}

func GetRedisSlaveName(rf *redisfailoverv1.RedisFailover) string {
	return generateName(redisSlaveName, rf.Name)
}

func generateName(typeName, metaName string) string {
	return fmt.Sprintf("%s%s-%s", baseName, typeName, metaName)
}

// GetPodDNSName returns the DNS name for a StatefulSet pod when headless is enabled
// Format: <statefulset-name>-<ordinal-number>.<service-name>.<namespace>.svc.cluster.local
func GetPodDNSName(pod *corev1.Pod, rf *redisfailoverv1.RedisFailover) string {
	if !rf.Spec.Redis.Headless {
		return pod.Status.PodIP
	}

	// Extract ordinal number from pod name (format: <statefulset-name>-<ordinal>)
	// For example: rfr-redis-redisfailover-0 -> 0
	parts := strings.Split(pod.Name, "-")
	if len(parts) == 0 {
		return pod.Status.PodIP
	}
	ordinalStr := parts[len(parts)-1]

	// Validate ordinal is a number
	if _, err := strconv.Atoi(ordinalStr); err != nil {
		return pod.Status.PodIP
	}

	statefulSetName := GetRedisName(rf)
	serviceName := GetRedisName(rf)
	namespace := rf.Namespace

	return fmt.Sprintf("%s-%s.%s.%s.svc.cluster.local", statefulSetName, ordinalStr, serviceName, namespace)
}

// isPodReady checks if a pod is in Ready state
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

// GetPodAddress returns either the DNS name (if headless and pod is ready) or PodIP
// DNS names for headless services are only available when pods are Ready
func GetPodAddress(pod *corev1.Pod, rf *redisfailoverv1.RedisFailover) string {
	if rf.Spec.Redis.Headless && isPodReady(pod) && pod.Status.PodIP != "" {
		// Only use DNS names when pod is Ready and has an IP
		// This ensures DNS records are available in the cluster
		return GetPodDNSName(pod, rf)
	}
	// Fall back to PodIP if headless is disabled, pod is not ready, or no IP yet
	return pod.Status.PodIP
}

// GetPodIPFromAddress takes an address (DNS name or IP) and returns the PodIP
// This is needed for Sentinel monitoring which only accepts IP addresses
func GetPodIPFromAddress(address string, rf *redisfailoverv1.RedisFailover, pods *corev1.PodList) string {
	// If it's already an IP address (doesn't contain ".svc.cluster.local"), return it
	if !strings.Contains(address, ".svc.cluster.local") {
		// Check if it's a valid IP format (has 3 dots)
		parts := strings.Split(address, ".")
		if len(parts) == 4 {
			// Likely an IP address, return as-is
			return address
		}
	}

	// It's a DNS name, find the matching pod and return its IP
	// We need to match by DNS name or by extracting the pod ordinal from the DNS name
	for _, pod := range pods.Items {
		// Try matching by the address we'd generate for this pod
		podAddress := GetPodAddress(&pod, rf)
		if podAddress == address {
			return pod.Status.PodIP
		}

		// Also try matching by DNS name directly (in case pod isn't ready yet)
		// Extract ordinal from DNS name: rfr-redisfailover-0.rfr-redisfailover.basic.svc.cluster.local -> 0
		if strings.Contains(address, ".svc.cluster.local") {
			dnsParts := strings.Split(address, ".")
			if len(dnsParts) > 0 {
				hostnamePart := dnsParts[0] // e.g., "rfr-redisfailover-0"
				hostnameParts := strings.Split(hostnamePart, "-")
				if len(hostnameParts) > 0 {
					ordinalStr := hostnameParts[len(hostnameParts)-1]
					// Extract ordinal from pod name
					podNameParts := strings.Split(pod.Name, "-")
					if len(podNameParts) > 0 && podNameParts[len(podNameParts)-1] == ordinalStr {
						return pod.Status.PodIP
					}
				}
			}
		}
	}

	// If we can't find it, return the address as-is (fallback)
	return address
}
