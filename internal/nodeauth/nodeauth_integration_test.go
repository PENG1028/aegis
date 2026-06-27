package nodeauth

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"aegis/internal/id"
	"aegis/internal/node"
)

// createTestDB creates an in-memory SQLite database with the required tables.
func createTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Create nodes table (simplified version of migration 010 + 028)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			node_id TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL DEFAULT 'worker',
			status TEXT NOT NULL DEFAULT 'unknown',
			hostname TEXT NOT NULL,
			local_ip TEXT NOT NULL DEFAULT '127.0.0.1',
			private_ip TEXT DEFAULT '',
			public_ip TEXT DEFAULT '',
			region TEXT NOT NULL DEFAULT '',
			network_id TEXT NOT NULL DEFAULT '',
			os TEXT NOT NULL DEFAULT '',
			arch TEXT NOT NULL DEFAULT '',
			agent_version TEXT NOT NULL DEFAULT '',
			last_heartbeat_at TEXT DEFAULT '',
			last_error TEXT DEFAULT '',
			is_current INTEGER NOT NULL DEFAULT 0,
			is_leader INTEGER NOT NULL DEFAULT 0,
			leader_elected_at TEXT DEFAULT '',
			ip_migrated INTEGER NOT NULL DEFAULT 0,
			state_version INTEGER NOT NULL DEFAULT 0,
			capabilities TEXT NOT NULL DEFAULT '{}',
			last_seen TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create nodes table: %v", err)
	}

	// Create node_join_tokens table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS node_join_tokens (
			id TEXT PRIMARY KEY,
			token_hash TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			allowed_roles TEXT NOT NULL DEFAULT '[]',
			expected_node_name TEXT NOT NULL DEFAULT '',
			allowed_source_cidr TEXT NOT NULL DEFAULT '',
			expires_at TEXT NOT NULL,
			used_at TEXT DEFAULT '',
			used_by_node_id TEXT DEFAULT '',
			revoked_at TEXT DEFAULT '',
			created_at TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create join_tokens table: %v", err)
	}

	// Create node_credentials table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS node_credentials (
			id TEXT PRIMARY KEY,
			node_id TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			created_at TEXT NOT NULL,
			last_used_at TEXT DEFAULT '',
			revoked_at TEXT DEFAULT ''
		)
	`)
	if err != nil {
		t.Fatalf("create node_credentials table: %v", err)
	}

	return db
}

func TestCreateJoinTokenStoresHashOnly(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)

	rawToken := id.GenerateRandomHex(32)
	tokenHash := hashToken(rawToken)

	tok := &JoinToken{
		ID:          id.New("jt"),
		TokenHash:   tokenHash,
		Name:        "test-token",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
		CreatedAt:   time.Now(),
	}

	if err := repo.CreateJoinToken(tok); err != nil {
		t.Fatalf("create join token: %v", err)
	}

	// Verify we can find by hash
	found, err := repo.FindJoinTokenByHash(tokenHash)
	if err != nil {
		t.Fatalf("find by hash: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find token by hash")
	}
	if found.ID != tok.ID {
		t.Errorf("expected id %s, got %s", tok.ID, found.ID)
	}
	if found.TokenHash != tokenHash {
		t.Errorf("expected hash %s, got %s", tokenHash, found.TokenHash)
	}

	// Verify raw token is NOT in DB (we can't find by raw token)
	found2, err := repo.FindJoinTokenByHash(hashToken("wrong-token"))
	if err != nil {
		t.Fatalf("find by wrong hash: %v", err)
	}
	if found2 != nil {
		t.Error("expected nil for wrong token hash")
	}
}

func TestExpiredJoinTokenRejected(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)

	now := time.Now()
	rawToken := id.GenerateRandomHex(32)
	tokenHash := hashToken(rawToken)

	tok := &JoinToken{
		ID:        id.New("jt"),
		TokenHash: tokenHash,
		Name:      "expired-token",
		ExpiresAt: now.Add(-1 * time.Hour), // expired
		CreatedAt: now.Add(-2 * time.Hour),
	}

	if err := repo.CreateJoinToken(tok); err != nil {
		t.Fatalf("create join token: %v", err)
	}

	found, err := repo.FindJoinTokenByHash(tokenHash)
	if err != nil {
		t.Fatalf("find by hash: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find token")
	}
	if found.IsValid() {
		t.Error("expected expired token to be invalid")
	}
}

func TestUsedJoinTokenRejected(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)

	now := time.Now()
	rawToken := id.GenerateRandomHex(32)
	tokenHash := hashToken(rawToken)

	tok := &JoinToken{
		ID:        id.New("jt"),
		TokenHash: tokenHash,
		Name:      "used-token",
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now,
	}

	if err := repo.CreateJoinToken(tok); err != nil {
		t.Fatalf("create join token: %v", err)
	}

	// Mark as used
	if err := repo.MarkJoinTokenUsed(tok.ID, "nd_test", now); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	found, err := repo.FindJoinTokenByHash(tokenHash)
	if err != nil {
		t.Fatalf("find by hash: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find token")
	}
	if found.IsValid() {
		t.Error("expected used token to be invalid")
	}
	if found.UsedByNodeID != "nd_test" {
		t.Errorf("expected used_by_node_id nd_test, got %s", found.UsedByNodeID)
	}
}

func TestRevokedJoinTokenRejected(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)

	now := time.Now()
	rawToken := id.GenerateRandomHex(32)
	tokenHash := hashToken(rawToken)

	tok := &JoinToken{
		ID:        id.New("jt"),
		TokenHash: tokenHash,
		Name:      "revoked-token",
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now,
	}

	if err := repo.CreateJoinToken(tok); err != nil {
		t.Fatalf("create join token: %v", err)
	}

	if err := repo.RevokeJoinToken(tok.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	found, err := repo.FindJoinTokenByHash(tokenHash)
	if err != nil {
		t.Fatalf("find by hash: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find token")
	}
	if found.IsValid() {
		t.Error("expected revoked token to be invalid")
	}
}

func TestNodeCredentialCreateAndFind(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)

	rawToken := id.GenerateRandomHex(32)
	tokenHash := hashToken(rawToken)

	cred := &NodeCredential{
		ID:        id.New("nc"),
		NodeID:    "nd_test",
		TokenHash: tokenHash,
		CreatedAt: time.Now(),
	}

	if err := repo.CreateNodeCredential(cred); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	// Find by node ID
	found, err := repo.FindNodeCredentialByNodeID("nd_test")
	if err != nil {
		t.Fatalf("find by node id: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find credential")
	}
	if found.NodeID != "nd_test" {
		t.Errorf("expected node_id nd_test, got %s", found.NodeID)
	}

	// Find by token hash
	found2, err := repo.FindNodeCredentialByTokenHash(tokenHash)
	if err != nil {
		t.Fatalf("find by hash: %v", err)
	}
	if found2 == nil {
		t.Fatal("expected to find credential by hash")
	}
}

func TestNodeCredentialRevoked(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)

	rawToken := id.GenerateRandomHex(32)
	tokenHash := hashToken(rawToken)

	cred := &NodeCredential{
		ID:        id.New("nc"),
		NodeID:    "nd_test2",
		TokenHash: tokenHash,
		CreatedAt: time.Now(),
	}

	if err := repo.CreateNodeCredential(cred); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	// Revoke
	if err := repo.RevokeNodeCredential(cred.ID); err != nil {
		t.Fatalf("revoke credential: %v", err)
	}

	// Should not find active credential
	found, err := repo.FindNodeCredentialByNodeID("nd_test2")
	if err != nil {
		t.Fatalf("find by node id: %v", err)
	}
	if found != nil {
		t.Error("expected nil for revoked credential")
	}

	// Should still find by hash but with revoked flag
	found2, err := repo.FindNodeCredentialByTokenHash(tokenHash)
	if err != nil {
		t.Fatalf("find by hash: %v", err)
	}
	if found2 != nil {
		t.Error("expected nil for revoked credential by hash")
	}
}

func TestListJoinTokens(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)

	now := time.Now()
	for i := 0; i < 3; i++ {
		tok := &JoinToken{
			ID:        id.New("jt"),
			TokenHash: hashToken(id.GenerateRandomHex(32)),
			Name:      "token",
			ExpiresAt: now.Add(1 * time.Hour),
			CreatedAt: now,
		}
		if err := repo.CreateJoinToken(tok); err != nil {
			t.Fatalf("create token %d: %v", i, err)
		}
	}

	tokens, err := repo.ListJoinTokens()
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	if len(tokens) != 3 {
		t.Errorf("expected 3 tokens, got %d", len(tokens))
	}
}

func TestServiceCreateAndValidateJoinToken(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	nodeRepo := node.NewRepository(db)
	nodeSvc := node.NewService(nodeRepo)
	svc := NewService(repo, nodeRepo, nodeSvc)

	// Create join token
	jt, rawToken, err := svc.CreateJoinToken(CreateJoinTokenInput{
		Name:             "test-join",
		AllowedRoles:     []string{"gateway", "worker"},
		ExpectedNodeName: "server-c",
		ExpiresInSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("create join token: %v", err)
	}
	if jt == nil {
		t.Fatal("expected non-nil join token")
	}
	if rawToken == "" {
		t.Fatal("expected non-empty raw token")
	}

	// Validate the join token
	validated, err := svc.ValidateJoinToken(rawToken, JoinRequest{
		NodeName: "server-c",
		Roles:    []string{"gateway"},
	}, "10.0.0.1")
	if err != nil {
		t.Fatalf("validate join token: %v", err)
	}
	if validated == nil {
		t.Fatal("expected validated token")
	}
	if !validated.IsValid() {
		t.Error("expected validated token to be valid")
	}
}

func TestServiceRoleMismatchRejected(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	nodeRepo := node.NewRepository(db)
	nodeSvc := node.NewService(nodeRepo)
	svc := NewService(repo, nodeRepo, nodeSvc)

	_, rawToken, err := svc.CreateJoinToken(CreateJoinTokenInput{
		Name:             "gateway-only",
		AllowedRoles:     []string{"gateway"},
		ExpiresInSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("create join token: %v", err)
	}

	// Try to join with disallowed role
	_, err = svc.ValidateJoinToken(rawToken, JoinRequest{
		NodeName: "server-x",
		Roles:    []string{"worker"}, // not in allowed roles
	}, "10.0.0.1")
	if err == nil {
		t.Error("expected error for role mismatch")
	}
}

func TestServiceExpectedNodeNameMismatch(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	nodeRepo := node.NewRepository(db)
	nodeSvc := node.NewService(nodeRepo)
	svc := NewService(repo, nodeRepo, nodeSvc)

	_, rawToken, err := svc.CreateJoinToken(CreateJoinTokenInput{
		Name:             "server-c-only",
		ExpectedNodeName: "server-c",
		ExpiresInSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("create join token: %v", err)
	}

	_, err = svc.ValidateJoinToken(rawToken, JoinRequest{
		NodeName: "server-d", // doesn't match
		Roles:    []string{"worker"},
	}, "10.0.0.1")
	if err == nil {
		t.Error("expected error for node name mismatch")
	}
}

func TestServiceFullRegistrationFlow(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	nodeRepo := node.NewRepository(db)
	nodeSvc := node.NewService(nodeRepo)
	svc := NewService(repo, nodeRepo, nodeSvc)

	// 1. Admin creates join token
	_, rawToken, err := svc.CreateJoinToken(CreateJoinTokenInput{
		Name:             "server-c-bootstrap",
		AllowedRoles:     []string{"gateway", "worker", "relay"},
		ExpectedNodeName: "server-c",
		ExpiresInSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("create join token: %v", err)
	}

	// 2. Node registers
	resp, err := svc.RegisterNode(JoinRequest{
		JoinToken:    rawToken,
		NodeName:     "server-c",
		Roles:        []string{"gateway", "relay"},
		Hostname:     "server-c.example.com",
		OS:           "linux",
		Arch:         "amd64",
		AgentVersion: "v1.8C",
		PublicIP:     "<SERVER_A_NODE_IP>",
		PrivateIP:    "10.0.0.3",
	}, "<CONTROL_PLANE_IP>")
	if err != nil {
		t.Fatalf("register node: %v", err)
	}
	if resp.NodeID == "" {
		t.Fatal("expected non-empty node_id")
	}
	if resp.NodeToken == "" {
		t.Fatal("expected non-empty node_token")
	}
	if resp.Status != "registered" {
		t.Errorf("expected status 'registered', got '%s'", resp.Status)
	}

	// 3. Verify node was created
	nodeRecord, err := nodeRepo.FindByNodeID(resp.NodeID)
	if err != nil {
		t.Fatalf("find node: %v", err)
	}
	if nodeRecord == nil {
		t.Fatal("expected node to exist")
	}
	if nodeRecord.Name != "server-c" {
		t.Errorf("expected node name 'server-c', got '%s'", nodeRecord.Name)
	}
	if nodeRecord.OS != "linux" {
		t.Errorf("expected OS 'linux', got '%s'", nodeRecord.OS)
	}

	// 4. Verify credential works
	authenticatedNodeID, err := svc.AuthenticateNode(resp.NodeToken)
	if err != nil {
		t.Fatalf("authenticate node: %v", err)
	}
	if authenticatedNodeID != resp.NodeID {
		t.Errorf("expected node_id %s, got %s", resp.NodeID, authenticatedNodeID)
	}

	// 5. Verify wrong token fails
	_, err = svc.AuthenticateNode("wrong-token")
	if err == nil {
		t.Error("expected auth error for wrong token")
	}

	// 6. Verify join token is now used
	_, err = svc.RegisterNode(JoinRequest{
		JoinToken: rawToken,
		NodeName:  "server-c-dup",
		Roles:     []string{"worker"},
	}, "10.0.0.1")
	if err == nil {
		t.Error("expected error for reusing join token")
	}
}

func TestJoinTokenRawReturnedOnce(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	nodeRepo := node.NewRepository(db)
	nodeSvc := node.NewService(nodeRepo)
	svc := NewService(repo, nodeRepo, nodeSvc)

	// Create token - raw token returned once
	jt, rawToken, err := svc.CreateJoinToken(CreateJoinTokenInput{
		Name:             "once-test",
		ExpiresInSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("create join token: %v", err)
	}
	if rawToken == "" {
		t.Fatal("expected raw token on creation")
	}

	// List should NOT contain raw token
	tokens, err := svc.ListJoinTokens()
	if err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	for _, t2 := range tokens {
		if t2.ID == jt.ID {
			// Token hash should not equal raw token
			if t2.TokenHash == rawToken {
				t.Error("list should not contain raw token")
			}
		}
	}
}

func TestNodeCredentialRawReturnedOnce(t *testing.T) {
	db := createTestDB(t)
	defer db.Close()
	repo := NewRepository(db)
	nodeRepo := node.NewRepository(db)
	nodeSvc := node.NewService(nodeRepo)
	svc := NewService(repo, nodeRepo, nodeSvc)

	_, rawJT, err := svc.CreateJoinToken(CreateJoinTokenInput{
		Name:             "cred-test",
		ExpiresInSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("create join token: %v", err)
	}

	// Register returns raw node token once
	resp, err := svc.RegisterNode(JoinRequest{
		JoinToken: rawJT,
		NodeName:  "cred-node",
		Roles:     []string{"worker"},
		Hostname:  "cred-node-host",
	}, "10.0.0.1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if resp.NodeToken == "" {
		t.Fatal("expected raw node token on registration")
	}

	// After registration, credential stored as hash
	cred, err := repo.FindNodeCredentialByNodeID(resp.NodeID)
	if err != nil {
		t.Fatalf("find credential: %v", err)
	}
	if cred == nil {
		t.Fatal("expected credential to exist")
	}
	if cred.TokenHash == resp.NodeToken {
		t.Error("DB should store hash, not raw token")
	}
	// Hash should match hash of raw token
	expectedHash := hashToken(resp.NodeToken)
	if cred.TokenHash != expectedHash {
		t.Error("stored hash should equal hash of raw token")
	}
}
