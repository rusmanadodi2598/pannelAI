//go:build integration

// Package postgres implements the repository contracts against PostgreSQL.
//
// @file      internal/repository/postgres/provider_node_delete_cascade_integration_test.go
// @for       The atomicity of the erase that takes a custom provider node, its endpoints, and its model rows.
// @uses      internal/domain, migrations, pgxpool, context, errors, testing, time.
// @reason    None of the three references is a foreign key, so the only thing keeping a deleted node's connections from outliving it is this transaction. Whether it commits whole and rolls back whole is a server property no fake can run, and the second one is the failure a node delete is most likely to hit: the node row gone before the cascade reached it.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     repository
// @stability stable
// @since     2026-10-16
package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
	"github.com/rusmanadodi2598/pannelAI/app-serv/migrations"
)

// newCascadeRepos opens one pool over the four tables the node erase touches and
// returns the node and endpoint repositories sharing it, so one transaction spans
// both aggregates the way the production pool does.
func newCascadeRepos(t *testing.T) (*NodeRepository, *EndpointRepository, *pgxpool.Pool) {
	t.Helper()

	dsn := requireTestDSN(t)
	ctx := context.Background()
	if err := migrations.Apply(ctx, dsn); err != nil {
		t.Fatalf("applying migrations: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(pool.Close)

	for _, statement := range []string{
		`TRUNCATE upstream_endpoints CASCADE`,
		`TRUNCATE models_custom`,
		`TRUNCATE models_disabled`,
		`TRUNCATE provider_nodes`,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("preparing the schema: %v", err)
		}
	}
	return NewNodeRepository(pool), NewEndpointRepository(pool), pool
}

// storeRollbackNode seeds one node with an endpoint and key and a custom model row
// under its id, and returns the two ids the assertions read.
func storeRollbackNode(t *testing.T, nodes *NodeRepository, endpoints *EndpointRepository, pool *pgxpool.Pool, id, prefix string) (string, string) {
	t.Helper()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond).Add(time.Second)
	node, err := domain.NewProviderNode(id, id, prefix, domain.NodeOpenAICompatible, domain.NodeAPIChat, "https://"+prefix+".example", now)
	if err != nil {
		t.Fatalf("building node %s: %v", id, err)
	}
	if err := nodes.Create(ctx, node); err != nil {
		t.Fatalf("storing node %s: %v", id, err)
	}
	endpointID := "ep-" + prefix
	storeKeyedEndpoint(t, endpoints, ctx, endpointID, node.ID(), now)
	storeCatalogModel(t, pool, ctx, "mc-"+prefix, node.ID(), "gpt-4o", now)
	return node.ID(), endpointID
}

func countProviderRows(t *testing.T, pool *pgxpool.Pool, table, providerID string) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM `+table+` WHERE provider_id = $1`, providerID).Scan(&count); err != nil {
		t.Fatalf("counting %s for %s: %v", table, providerID, err)
	}
	return count
}

func TestIntegration_DeleteCascadeTakesTheNodeAndEveryDependent(t *testing.T) {
	nodes, endpoints, pool := newCascadeRepos(t)
	ctx := context.Background()
	takenNodeID, endpointID := storeRollbackNode(t, nodes, endpoints, pool, "pn-taken", "taken")
	keptNodeID, otherID := storeRollbackNode(t, nodes, endpoints, pool, "pn-kept", "kept")

	if err := nodes.DeleteCascade(ctx, takenNodeID); err != nil {
		t.Fatalf("DeleteCascade(): %v", err)
	}

	if _, err := nodes.GetByID(ctx, takenNodeID); !errors.Is(err, domain.ErrNodeNotFound) {
		t.Fatalf("the node survived its own cascade: %v", err)
	}
	if _, err := endpoints.GetByID(ctx, endpointID); !errors.Is(err, domain.ErrEndpointNotFound) {
		t.Fatalf("endpoint %s survived the cascade: %v", endpointID, err)
	}
	if left := countStoredKeys(t, endpoints, ctx, endpointID); left != 0 {
		t.Fatalf("%d key row(s) outlived the cascade that erased their endpoint", left)
	}
	if left := countProviderRows(t, pool, "models_custom", takenNodeID); left != 0 {
		t.Fatalf("%d custom model row(s) survived the cascade", left)
	}

	if _, err := nodes.GetByID(ctx, keptNodeID); err != nil {
		t.Fatalf("the cascade reached another node: %v", err)
	}
	if _, err := endpoints.GetByID(ctx, otherID); err != nil {
		t.Fatalf("the cascade reached an endpoint of another node: %v", err)
	}
	if left := countProviderRows(t, pool, "models_custom", keptNodeID); left != 1 {
		t.Fatalf("the other node's model rows = %d, want its one row untouched", left)
	}
}

// TestIntegration_DeleteCascadeRollsBackWhenTheNodeRowIsGone is the F10 case: the
// node row disappears between the guard and the erase, and the last statement of the
// cascade fails. A cascade that ran as separate statements would have deleted the
// endpoints and the model rows already, leaving a provider with nothing stored under
// it and no way to explain why.
func TestIntegration_DeleteCascadeRollsBackWhenTheNodeRowIsGone(t *testing.T) {
	nodes, endpoints, pool := newCascadeRepos(t)
	ctx := context.Background()
	goneNodeID, endpointID := storeRollbackNode(t, nodes, endpoints, pool, "pn-gone", "gone")

	if err := nodes.Delete(ctx, goneNodeID); err != nil {
		t.Fatalf("removing the node row first: %v", err)
	}
	if err := nodes.DeleteCascade(ctx, goneNodeID); !errors.Is(err, domain.ErrNodeNotFound) {
		t.Fatalf("DeleteCascade() error = %v, want ErrNodeNotFound", err)
	}

	if _, err := endpoints.GetByID(ctx, endpointID); err != nil {
		t.Fatalf("the aborted cascade erased the endpoint: %v", err)
	}
	if left := countStoredKeys(t, endpoints, ctx, endpointID); left != 1 {
		t.Fatalf("the aborted cascade left %d key row(s), want its one key intact", left)
	}
	if left := countProviderRows(t, pool, "models_custom", goneNodeID); left != 1 {
		t.Fatalf("the aborted cascade erased the model rows: %d rows left, want 1", left)
	}
}
