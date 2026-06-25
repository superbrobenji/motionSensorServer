# Task 1 Code Review — Proto schema + nodeauth package + MeshServer enrollment wiring

> This section was written by the code reviewer. The original coding-agent self-report follows below the horizontal rule.

## Spec Compliance: PARTIAL

Core protocol constraints are satisfied. One Critical issue violates the global JoinAck constraint directly.

## Task Quality: Approved with Required Fix

The implementation is structurally sound and all three pre-existing bugs are correctly fixed. One Critical issue and two Important issues must be resolved before this branch is integrated.

---

## What Was Done Well

The per-node replay cache replacement is correct. The old flat shared ring buffer would have allowed a replay from one node to evict another node's entries; the new `map[[6]byte]*nodeBuf` in `/Users/benjamin.swanepoel/projects/personal/motionSensorServer/.worktrees/full-review/server/orchistrator/nodeauth/replay.go` gives each node a fully independent 32-slot ring with FIFO eviction.

The proto field ordering correction in `mesh.proto` and regenerated `mesh.pb.go` is accurate: `epochNum=8, seqNum=9, protoVersion=10` now matches the firmware. The rawDesc binary in `mesh.pb.go` has been regenerated (protoc v7.35.1 was used per the header), resolving the earlier coding agent concern that the rawDesc was hand-maintained.

The enrollment delivery pivot from raw opcode bytes to a first-class `MeshMessage(messageType=2)` is a well-reasoned architectural improvement. It eliminates dual-framing ambiguity and keeps all wire traffic within the protobuf envelope.

`persistence.go` Load() correctly restores `ReceivedAt` and `ApprovedAt` from Unix timestamps (the prior omission was a regression bug).

The `stopPersist` lifecycle (created in `NewMeshServer`, started in `Start()`, closed in `Stop()`, with a final save in `PersistLoop` on channel close) is correct and race-free.

Build passes cleanly: `go build ./...` exits 0 with no warnings or errors.

---

## Findings

### Critical

**C1 — ApproveEnrollment sends the node's own public key back, violating the JoinAck constraint**

The global constraints state: `JoinAck (approval): MeshMessage(messageType=3, originMacAddress=node_mac, public_key=32B non-zero)`

"32B non-zero" in this context is the server's Curve25519 public key delivered to the node for ECDH derivation. The current implementation echoes the node's own public key back:

File: `/Users/benjamin.swanepoel/projects/personal/motionSensorServer/.worktrees/full-review/server/orchistrator/mesh/server.go` lines 385–390

```go
approvalMsg := &MeshMessage{
    MessageType:      MessageTypeJoinAck,
    OriginMacAddress: node.MAC[:],
    PublicKey:        node.PublicKey[:],   // node's own key, not server's key
}
```

The firmware expects the server's Curve25519 public key so it can compute the shared secret. Echoing the node's own key gives the firmware its own public key, producing a nonsense ECDH shared secret. The node will record the key and move on without error, meaning this will fail silently during encrypted message exchange.

Task 1 scope does not include generating a server keypair (that is a later task), but the current code must not silently ship a broken exchange. Required resolution: either block `ApproveEnrollment` with an error until a server keypair is configured, or add a clearly visible TODO that marks this as incomplete and prevents it from being called in production. The current state will produce a hard-to-diagnose silent encryption failure.

---

### Important

**I1 — handlePIRData will nil-dereference when eventStore is nil**

`logMessageToKafka` correctly guards against a nil event store (line 535), but `handlePIRData` calls `ms.eventStore.WriteMessage` directly without a nil check (line 365). When the server starts without Kafka — which `main.go` explicitly permits and logs as a normal degraded mode — any incoming PIR message will cause a nil pointer dereference and terminate the `messageProcessor` goroutine.

File: `/Users/benjamin.swanepoel/projects/personal/motionSensorServer/.worktrees/full-review/server/orchistrator/mesh/server.go` line 365

This diff touched `handlePIRData` to add proper `json.Marshal` error handling, making this an expected fix that was missed. The guard pattern is already present in `handleEnrollmentRequest` at lines 338–341.

**I2 — OriginMacAddress length not validated before copy in replay path**

In `handleMessage`, the replay check copies `OriginMacAddress` into a `[6]byte` without first confirming the slice is at least 6 bytes (line 213). Go's `copy` silently copies fewer bytes if the source is short, producing a truncated MAC key in the replay map that will never match the same node's full-length key in subsequent messages, breaking per-node replay deduplication.

File: `/Users/benjamin.swanepoel/projects/personal/motionSensorServer/.worktrees/full-review/server/orchistrator/mesh/server.go` line 212–213

`handleEnrollmentRequest` correctly checks `len(msg.OriginMacAddress) < 6` before its equivalent copy (line 305). The same guard is needed here.

**I3 — No tests for the nodeauth package**

The `nodeauth` directory contains no `_test.go` files. The registry operations (`AddPending`, `Approve`, `Reject`), the persistence round-trip (Persist then Load), and the replay cache's per-node isolation are all security-critical paths with non-trivial logic. The coding agent's own self-report flags this as a known gap. The persistence round-trip in particular must have a test confirming that timestamps, status values, and key material survive a full cycle. The replay cache FIFO ring eviction and zero-value guard in `IsDuplicate` also need test coverage.

---

### Minor

**M1 — NewReplayCache silently ignores its size argument**

`NewReplayCache(_ int)` is called with `64` from `NewMeshServer` but every per-node buffer is hard-coded to `[32]ringEntry`. The minimum 32-entry requirement is met, but the API is misleading: a caller passing `128` silently gets `32`. Either remove the parameter and document the fixed size, or honour the argument.

File: `/Users/benjamin.swanepoel/projects/personal/motionSensorServer/.worktrees/full-review/server/orchistrator/nodeauth/replay.go` line 22

**M2 — handleEnrollmentRequest returns an error on re-enrollment of an approved node**

If an already-approved node re-sends an enrollment message after a reboot, `AddPending` returns `"node %s already approved"` and `handleEnrollmentRequest` propagates this as a processing failure. This is an expected operational event, not a real failure. It should be demoted to a log message with a `return nil`.

File: `/Users/benjamin.swanepoel/projects/personal/motionSensorServer/.worktrees/full-review/server/orchistrator/mesh/server.go` lines 319–323; `/Users/benjamin.swanepoel/projects/personal/motionSensorServer/.worktrees/full-review/server/orchistrator/nodeauth/registry.go` lines 43–44

**M3 — Duplicate wire value 3 across two const blocks**

`MessageTypeSerialCmdBroadcast = 3` and `MessageTypeJoinAck = 3` share the same integer value in different `const` blocks. The note in `constants.go` line 8 documents the directional disambiguation correctly and is adequate for Task 1. The real fix (assigning non-colliding values in firmware) is out of scope here. No action required this task.

---

### Info

**Info1 — Plan deviation: enrollment via MeshMessage instead of raw 0xC0 opcode**

The original plan (Phase 3 Step 3) specified enrollment via `OP_ENROLLMENT_REQ=0xC0` opcode frames parsed from `msg.Data`. The implementation uses `messageType=2` as a first-class `MeshMessage` field with `public_key=11`. The task brief explicitly called this out as a required correction. The deviation is approved.

**Info2 — WriteRaw not added to serial.go**

The plan's Phase 3 Step 4 prescribed adding `WriteRaw` to `SerialComm`. It was not added. Instead, `ApproveEnrollment` uses `ms.serialComm.WriteFrame` (protobuf write), which is consistent with the enrollment-via-protobuf deviation. `WriteRaw` is unnecessary for this task scope.

**Info3 — macToString format inconsistency between packages**

`nodeauth/registry.go` produces bare hex (`aabbccddeeff`) while `mesh/node_registry.go` produces colon-separated format (`aa:bb:cc:dd:ee:ff`). No current integration path crosses these representations. Track for future unification.

---

## Finding Summary

| ID | Severity  | Location | Finding |
|----|-----------|----------|---------|
| C1 | Critical  | server.go:385–390 | ApproveEnrollment echoes node's own public key; server keypair needed per JoinAck constraint |
| I1 | Important | server.go:365 | handlePIRData dereferences nil eventStore when Kafka is unavailable |
| I2 | Important | server.go:212–213 | OriginMacAddress length unchecked before copy in replay path |
| I3 | Important | nodeauth/ | No tests for nodeauth package (registry, persistence, replay cache) |
| M1 | Minor     | replay.go:22 | NewReplayCache size param silently ignored; per-node buffer hard-coded to 32 |
| M2 | Minor     | server.go:319 | Re-enrollment of approved node treated as error instead of info log |
| M3 | Minor     | constants.go:7,30 | Duplicate wire value 3 — documented in comment, fix is out of scope |

---

## Original Coding Agent Self-Report

*(The section below was written by the coding agent and is preserved for traceability.)*

---

# Task 1 Report — Proto schema + nodeauth package + MeshServer enrollment wiring

## Status: DONE_WITH_CONCERNS

---

## Commits (in order)

| Hash | Message |
|------|---------|
| `0e801de` | feat: extend mesh.proto with security fields (protoVersion, epochNum, seqNum, enrollment types) |
| `4746916` | feat: nodeauth package — trusted node registry with persistence, replay cache |
| `6c06e25` | feat: enrollment handling in MeshServer — pending registry, approval/rejection, serial OP_ENROLLMENT_APPROVE dispatch |

---

## Files Created or Modified

### Phase 1 — Protocol Schema Update

| File | Action |
|------|--------|
| `server/orchistrator/mesh/mesh.proto` | Modified — added fields 8-10 (protoVersion, epochNum, seqNum) to MeshMessage; added EnrollmentRequest and EnrollmentApproval messages |
| `server/orchistrator/mesh/mesh.pb.go` | Modified — hand-maintained to add Go struct fields and getters for protoVersion/epochNum/seqNum; added EnrollmentRequest and EnrollmentApproval structs |

### Phase 2 — Node Auth Package

| File | Action |
|------|--------|
| `server/orchistrator/nodeauth/registry.go` | Created — TrustStatus enum, NodeAuth struct, Registry with AddPending/Approve/Reject/GetAll/GetPending/IsApproved/GetApprovedPublicKey |
| `server/orchistrator/nodeauth/persistence.go` | Created — Persist/Load/PersistLoop (JSON flat-file, 0600 perms) |
| `server/orchistrator/nodeauth/replay.go` | Created — ReplayCache with 64-entry ring buffer, IsDuplicate() |

### Phase 3 — MeshServer Integration

| File | Action |
|------|--------|
| `server/orchistrator/mesh/server.go` | Modified — authRegistry/replayCache/authPath/stopPersist fields; updated NewMeshServer; Start() launches PersistLoop; Stop() closes stopPersist; handleMessage() adds version check + replay detection; handleSerialData() routes 0xC0; handleEnrollmentRequest(); ApproveEnrollment(); RejectEnrollment(); GetPendingEnrollments(); GetAuthRegistry() |
| `server/orchistrator/mesh/serial.go` | Modified — added WriteRaw() for raw opcode frame writes; added TryReadEnrollmentFrame() helper |
| `server/orchistrator/mesh/constants.go` | Modified — added OpEnrollmentReq/Approve/Reject (0xC0/C1/C2); added MessageTypeEnrollment=2 and MessageTypeJoinAck=3 (per task brief step) |
| `server/orchistrator/main.go` | Modified — added --auth-registry flag (default: data/nodeauth.json); os.MkdirAll("data"); AuthRegistryPath in MeshServerConfig |

---

## Security Plan Steps: Completed vs Skipped

### Phase 1
- [x] Step 1: Update mesh.proto — COMPLETED
- [~] Step 2: Regenerate Go protobuf — SKIPPED (see mesh.pb.go note below)
- [x] Step 3: Commit — COMPLETED (hash: 0e801de)

### Phase 2
- [x] Step 1: Write nodeauth/registry.go — COMPLETED (with correction: curveToEdPublicKey placeholder removed per task brief)
- [x] Step 2: Write nodeauth/persistence.go — COMPLETED
- [x] Step 3: Write nodeauth/replay.go — COMPLETED
- [~] Step 4: Compile check — SKIPPED (go not available in shell)
- [x] Step 5: Commit — COMPLETED (hash: 4746916)

### Phase 3
- [x] Step 1: Add nodeauth to MeshServer — COMPLETED
- [x] Step 2: Proto version + replay check in handleMessage() — COMPLETED
- [x] Step 3: Handle enrollment serial opcode 0xC0 — COMPLETED
- [x] Step 4: ApproveEnrollment() and RejectEnrollment() — COMPLETED (using WriteRaw as instructed)
- [x] Step 5: Wire AuthRegistryPath in main.go — COMPLETED
- [~] Step 6: Compile and run — SKIPPED (go not available in shell)
- [x] Step 7: Commit — COMPLETED (hash: 6c06e25)

### Task Brief post-steps
- [x] Add MESH_TYPE_ENROLLMENT = 2 and MESH_TYPE_JOIN_ACK = 3 to constants.go — COMPLETED (as MessageTypeEnrollment and MessageTypeJoinAck)
- [~] Step 2: Compile check — SKIPPED (go not available in shell)
- [x] Step 3: Commit — included in Phase 3 commit (6c06e25)

---

## mesh.pb.go: Regenerated or Left As-Is?

**Hand-maintained (not regenerated with protoc).**

Reason: `protoc` was not available in the shell environment.

The existing `mesh.pb.go` only had `MeshMessage` with fields 1-7 and a single-message rawDesc. The updated file:
- Adds `ProtoVersion`, `EpochNum`, `SeqNum` fields with correct protobuf tags to `MeshMessage`
- Adds full `EnrollmentRequest` and `EnrollmentApproval` structs with getters
- Updates `file_mesh_mesh_proto_msgTypes` from size 1 to 3
- Updates `file_mesh_mesh_proto_goTypes` to include all three message types
- Keeps the `rawDesc` string updated to match the new schema

**IMPORTANT:** The `rawDesc` constant in the hand-maintained file is the critical concern (see Concerns below). It must be regenerated with `protoc` before this is used in production. The Go struct fields and getters are correct and will work for direct struct usage, but the rawDesc binary descriptor (used by protobuf reflection and wire-format registration) needs proper protoc output to be guaranteed correct.

---

## Deviations from Plan

1. **curveToEdPublicKey removed**: Per task brief correction, the `curveToEdPublicKey` placeholder function in `registry.go` was omitted entirely (it's unused). The `crypto/ed25519` import was therefore also not included.

2. **ApproveEnrollment uses WriteRaw**: Per task brief correction, `ApproveEnrollment` calls `ms.serialComm.WriteRaw(combined)` instead of the plan's `ms.serialComm.port.Write(combined)` (private field). `WriteRaw` was added to `serial.go`.

3. **PersistLoop guarded by authPath != ""**: Added a nil-path guard in `Start()` so `PersistLoop` is not started when `AuthRegistryPath` is empty (prevents a ticker that saves to a blank filename).

4. **Persist calls guarded by authPath != ""**: `handleEnrollmentRequest`, `ApproveEnrollment`, and `RejectEnrollment` all check `ms.authPath != ""` before calling `Persist`, matching the guard used in `Start()`.

5. **MessageTypeJoinAck = 3 collides with MessageTypeSerialCmdBroadcast = 3**: Both constants have value 3. The plan's proto comment reassigns 3 to JOIN_ACK. `MessageTypeSerialCmdBroadcast` is only used for outgoing broadcast commands (in `message_builder.go:BuildBroadcastMessage`), so there is no incoming-message dispatch collision. This is a pre-existing design issue in the security plan, not introduced by this implementation.

6. **constants.go carries both enrollment opcodes AND enrollment message type constants**: The task brief asked for `MessageTypeEnrollment` and `MessageTypeJoinAck` to be added to `constants.go`. The enrollment opcodes (`OpEnrollmentReq/Approve/Reject`) were also added to `constants.go` (referenced by `serial.go` and `server.go`) rather than just in `serial.go`, for consistency with existing opcode grouping.

---

## Concerns

1. **mesh.pb.go rawDesc is hand-maintained**: The binary protobuf descriptor string in `rawDesc` was manually constructed. While the Go struct fields are correct and can be used directly, the binary rawDesc drives protobuf reflection, JSON marshaling, and wire-format registration. An incorrect rawDesc will panic at startup when `file_mesh_mesh_proto_init()` calls `TypeBuilder.Build()`. **Action required:** Run `protoc --go_out=. --go_opt=paths=source_relative mesh/mesh.proto` from `server/orchistrator/` and replace mesh.pb.go with the generated output before deploying.

2. **Compile check not performed**: `go build ./...` was not run. All imports look correct (nodeauth package uses only stdlib; mesh/server.go imports `github.com/superbrobenji/motionServer/nodeauth` which is in the module). Manual compile check required before merging.

3. **No unit tests for nodeauth**: The plan did not include tests for the new package. Tests for `Registry`, `ReplayCache`, and `Persist/Load` should be added in a follow-up task.

---

## Fix Report

Fixes applied for C1, I1, I2, I3.

### C1
ApproveEnrollment now returns errors.New("server keypair not initialized: enrollment approval not yet supported") immediately. No JoinAck sent until server keypair is implemented. The old code echoed the node's own public key back in the JoinAck, which would have produced a nonsense ECDH shared secret on the firmware side without any error signal.

### I1
handlePIRData: nil guard added before eventStore use. When the server starts without Kafka (a documented degraded mode), PIR events are now dropped with a log line rather than causing a nil pointer dereference that would kill the messageProcessor goroutine.

### I2
handleMessage: len(msg.OriginMacAddress) != 6 check added before copy. Short or missing OriginMacAddress now causes the message to be dropped with a log line rather than silently producing a truncated MAC key in the replay map.

### I3
nodeauth_test.go created at server/orchistrator/nodeauth/nodeauth_test.go with 7 tests covering:
- TestRegistry_AddPendingAndGetPending
- TestRegistry_ApproveMovesToTrusted
- TestRegistry_GetAll
- TestPersistence_RoundTrip
- TestReplayCache_DuplicateDetection
- TestReplayCache_PerNodeIsolation
- TestReplayCache_RingWrap

Tests run:
```
=== RUN   TestRegistry_AddPendingAndGetPending
--- PASS: TestRegistry_AddPendingAndGetPending (0.00s)
=== RUN   TestRegistry_ApproveMovesToTrusted
--- PASS: TestRegistry_ApproveMovesToTrusted (0.00s)
=== RUN   TestRegistry_GetAll
--- PASS: TestRegistry_GetAll (0.00s)
=== RUN   TestPersistence_RoundTrip
--- PASS: TestPersistence_RoundTrip (0.00s)
=== RUN   TestReplayCache_DuplicateDetection
--- PASS: TestReplayCache_DuplicateDetection (0.00s)
=== RUN   TestReplayCache_PerNodeIsolation
--- PASS: TestReplayCache_PerNodeIsolation (0.00s)
=== RUN   TestReplayCache_RingWrap
--- PASS: TestReplayCache_RingWrap (0.00s)
PASS
ok  	github.com/superbrobenji/motionServer/nodeauth	0.466s
```

Commits:
- bc3ac36 fix: ApproveEnrollment returns error until server keypair implemented
- d42f507 fix: nil guard for eventStore in handlePIRData, MAC length check in handleMessage
- f2c2a06 test: nodeauth registry, persistence, replay cache isolation and deduplication
