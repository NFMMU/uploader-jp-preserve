package api

import (
	"testing"
	"time"
)

func TestNodeUtilization(t *testing.T) {
	cases := []struct {
		name      string
		used      int64
		capacity  int64
		requested int64
		want      float64
		ok        bool
	}{
		{"declared quota wins", 90, 1000, 100, 0.9, true},
		{"falls back to disk capacity", 90, 100, 0, 0.9, true},
		{"unknown capacity is not empty", 0, 0, 0, 0, false},
		{"negative capacity is not empty", 5, -1, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := nodeUtilization(tc.used, tc.capacity, tc.requested)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Fatalf("utilization = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSelectRebalanceSourceRequiresHotNode(t *testing.T) {
	warm := []rebalanceCopy{
		{PlacementID: "a", NodeID: "a", Utilization: 0.5, Usable: true},
		{PlacementID: "b", NodeID: "b", Utilization: 0.79, Usable: true},
	}
	if _, ok := selectRebalanceSource(warm); ok {
		t.Fatal("selected a source below the hot threshold")
	}
	copies := append(warm, rebalanceCopy{PlacementID: "c", NodeID: "c", Utilization: 0.85, Usable: true})
	source, ok := selectRebalanceSource(copies)
	if !ok || source.NodeID != "c" {
		t.Fatalf("source = %+v (ok=%v), want node c", source, ok)
	}
}

func TestRankRebalanceTargets(t *testing.T) {
	nodes := []rebalanceTarget{
		{ID: "hot", Utilization: 0.9, Usable: true, UsedBytes: 900, AvailableBytes: 100, CapacityBytes: 1000, RequestedCapacityBytes: 1000},
		{ID: "big-free", Utilization: 0.65, Usable: true, UsedBytes: 1300, AvailableBytes: 700, CapacityBytes: 2000, RequestedCapacityBytes: 2000},
		{ID: "small-free", Utilization: 0.3, Usable: true, UsedBytes: 60, AvailableBytes: 140, CapacityBytes: 200, RequestedCapacityBytes: 200},
		{ID: "occupied", Utilization: 0.1, Usable: true, UsedBytes: 100, AvailableBytes: 900, CapacityBytes: 1000, RequestedCapacityBytes: 1000},
		{ID: "unknown", Utilization: 0, Usable: false, AvailableBytes: 1000},
		{ID: "too-full", Utilization: 0.75, Usable: true, UsedBytes: 1500, AvailableBytes: 500, CapacityBytes: 2000, RequestedCapacityBytes: 2000},
	}
	occupied := map[string]bool{"occupied": true}
	ranked := rankRebalanceTargets(nodes, "hot", 900, 1000, occupied, 100)
	if len(ranked) != 2 || ranked[0].ID != "small-free" || ranked[1].ID != "big-free" {
		t.Fatalf("ranked = %+v, want small-free then big-free (both >= 20 points below the source)", ranked)
	}

	// A tighter gap excludes the node that is only 25 points below the source.
	ranked = rankRebalanceTargets(nodes, "hot", 800, 1000, occupied, 100)
	if len(ranked) != 1 || ranked[0].ID != "small-free" {
		t.Fatalf("ranked = %+v, want only small-free", ranked)
	}

	// A node that cannot fit the file is never a target.
	for _, target := range rankRebalanceTargets(nodes, "hot", 900, 1000, nil, 500) {
		if target.ID == "small-free" {
			t.Fatal("selected a target without room for the file")
		}
	}
}

func TestSelectSurplusVictimPrefersFullestNode(t *testing.T) {
	copies := []rebalanceCopy{
		{PlacementID: "newest", NodeID: "cold", Utilization: 0.2, Usable: true},
		{PlacementID: "oldest", NodeID: "hot", Utilization: 0.95, Usable: true},
		{PlacementID: "unknown", NodeID: "unknown", Usable: false},
	}
	victim, ok := selectSurplusVictim(copies)
	if !ok || victim.PlacementID != "oldest" {
		t.Fatalf("victim = %+v (ok=%v), want the copy on the hot node", victim, ok)
	}
}

func TestDeletionMinimumHealthy(t *testing.T) {
	if got := deletionMinimumHealthy(nil); got != requiredReplicaCount+1 {
		t.Fatalf("surplus minimum = %d, want %d", got, requiredReplicaCount+1)
	}
	now := time.Now()
	if got := deletionMinimumHealthy(&now); got != requiredReplicaCount {
		t.Fatalf("rebalance minimum = %d, want %d", got, requiredReplicaCount)
	}
	if deletionSafetySatisfied(2, 0, 1) != true {
		t.Fatal("minimum below the hard floor must be raised to 2")
	}
	if deletionSafetySatisfied(1, 0, requiredReplicaCount) {
		t.Fatal("one remaining copy must never be considered safe")
	}
	if deletionSafetySatisfied(3, 1, requiredReplicaCount) {
		t.Fatal("an unstable remaining copy must block deletion")
	}
}

func TestReserveManualReconcileSingleFlight(t *testing.T) {
	s := &Server{}
	first, ok := s.reserveManualReconcile("admin@example.invalid")
	if !ok || first == "" {
		t.Fatal("first manual run was not reserved")
	}
	if current, ok := s.reserveManualReconcile("admin@example.invalid"); ok || current != first {
		t.Fatalf("second manual reservation = (%q, %v), want the running run %q and false", current, ok, first)
	}
	if _, ok := s.reserveScheduledReconcile(); ok {
		t.Fatal("scheduled run was reserved while a manual run is pending")
	}
	s.finishReconcile(first, newPlacementReconcileStats(), nil)
	second, ok := s.reserveManualReconcile("admin@example.invalid")
	if !ok || second == first {
		t.Fatal("a new run must be allowed after the previous run finished")
	}
	if snapshot := s.reconcileSnapshot(""); snapshot["status"] != "running" {
		t.Fatalf("status = %v, want running", snapshot["status"])
	}
}

func TestRankRebalanceTargetsExactTwentyPointGap(t *testing.T) {
	nodes := []rebalanceTarget{
		{ID: "exact", Utilization: 0.65, Usable: true, UsedBytes: 650, AvailableBytes: 350, CapacityBytes: 1000, RequestedCapacityBytes: 1000},
		{ID: "over", Utilization: 0.651, Usable: true, UsedBytes: 651, AvailableBytes: 349, CapacityBytes: 1000, RequestedCapacityBytes: 1000},
	}
	ranked := rankRebalanceTargets(nodes, "hot", 850, 1000, map[string]bool{}, 1)
	if len(ranked) != 1 || ranked[0].ID != "exact" {
		t.Fatalf("ranked = %+v, want only the exact 20-point target", ranked)
	}
}
