// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.
//
// Author: Peter Mattis (peter@cockroachlabs.com)

package sql

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/cockroachdb/cockroach/pkg/config"
	"github.com/cockroachdb/cockroach/pkg/internal/client"
	"github.com/cockroachdb/cockroach/pkg/sql/mon"
	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	"github.com/cockroachdb/cockroach/pkg/util/envutil"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/pkg/errors"
)

// planner is the centerpiece of SQL statement execution combining session
// state and database state with the logic for SQL execution.
// A planner is generally part of a Session object. If one needs to be created
// outside of a Session, use makePlanner().
type planner struct {
	txn *client.Txn
	// As the planner executes statements, it may change the current user session.
	// TODO(andrei): see if the circular dependency between planner and Session
	// can be broken if we move the User and Database here from the Session.
	session  *Session
	semaCtx  parser.SemaContext
	evalCtx  parser.EvalContext
	leases   []*LeaseState
	leaseMgr *LeaseManager
	// This is used as a cache for database names.
	// TODO(andrei): get rid of it and replace it with a leasing system for
	// database descriptors.
	systemConfig  config.SystemConfig
	databaseCache *databaseCache

	testingVerifyMetadataFn func(config.SystemConfig) error
	verifyFnCheckedOnce     bool

	parser parser.Parser

	// If set, table descriptors will only be fetched at the time of the
	// transaction, not leased. This is used for things like AS OF SYSTEM TIME
	// queries and building query plans for views when they're created.
	// It's used in layers below the executor to modify the behavior of SELECT.
	avoidCachedDescriptors bool

	// If set, the planner should skip checking for the SELECT privilege when
	// initializing plans to read from a table. This should be used with care.
	skipSelectPrivilegeChecks bool

	// If set, contains the in progress COPY FROM columns.
	copyFrom *copyNode

	// Avoid allocations by embedding commonly used visitors.
	subqueryVisitor             subqueryVisitor
	subqueryPlanVisitor         subqueryPlanVisitor
	collectSubqueryPlansVisitor collectSubqueryPlansVisitor
	nameResolutionVisitor       nameResolutionVisitor

	execCfg *ExecutorConfig
}

// makePlanner creates a new planner instances, referencing a dummy Session.
// Only use this internally where a Session cannot be created.
func makePlanner(opName string) *planner {
	// init with an empty session. We can't leave this nil because too much code
	// looks in the session for the current database.
	ctx := log.WithLogTagStr(context.Background(), opName, "")
	p := &planner{
		session: &Session{
			Location: time.UTC,
			context:  ctx,
		},
	}
	p.session.TxnState.Ctx = ctx
	return p
}

// queryRunner abstracts the services provided by a planner object
// to the other SQL front-end components.
type queryRunner interface {
	// The following methods control the state of the planner during its
	// lifecycle.

	// setTxn  resets the current transaction in the planner and
	// initializes the timestamps used by SQL built-in functions from
	// the new txn object, if any.
	setTxn(*client.Txn)

	// resetTxn clears the planner's current transaction.
	resetTxn()

	// resetForBatch prepares the planner for executing a new batch of
	// statements.
	resetForBatch(e *Executor)

	// The following methods run SQL queries.

	// queryRow executes a SQL query string where exactly 1 result row is
	// expected and returns that row.
	queryRow(sql string, args ...interface{}) (parser.DTuple, error)

	// exec executes a SQL query string and returns the number of rows
	// affected.
	exec(sql string, args ...interface{}) (int, error)

	// The following methods can be used during testing.

	// setTestingVerifyMetadata sets a callback to be called after the planner
	// is done executing the current SQL statement. It can be used to verify
	// assumptions about how metadata will be asynchronously updated.
	// Note that this can overwrite a previous callback that was waiting to be
	// verified, which is not ideal.
	setTestingVerifyMetadata(fn func(config.SystemConfig) error)

	// blockConfigUpdatesMaybe will ask the Executor to block config updates,
	// so that checkTestingVerifyMetadataInitialOrDie() can later be run.
	// The point is to lock the system config so that no gossip updates sneak in
	// under us, so that we're able to assert that the verify callback only succeeds
	// after a gossip update.
	//
	// It returns an unblock function which can be called after
	// checkTestingVerifyMetadata{Initial}OrDie() has been called.
	//
	// This lock does not change semantics. Even outside of tests, the planner uses
	// static systemConfig for a user request, so locking the Executor's
	// systemConfig cannot change the semantics of the SQL operation being performed
	// under lock.
	blockConfigUpdatesMaybe(e *Executor) func()

	// checkTestingVerifyMetadataInitialOrDie verifies that the metadata callback,
	// if one was set, fails. This validates that we need a gossip update for it to
	// eventually succeed.
	// No-op if we've already done an initial check for the set callback.
	// Gossip updates for the system config are assumed to be blocked when this is
	// called.
	checkTestingVerifyMetadataInitialOrDie(e *Executor, stmts parser.StatementList)

	// checkTestingVerifyMetadataOrDie verifies the metadata callback, if one was
	// set.
	// Gossip updates for the system config are assumed to be blocked when this is
	// called.
	checkTestingVerifyMetadataOrDie(e *Executor, stmts parser.StatementList)
}

var _ queryRunner = &planner{}

// ctx returns the current session context (suitable for logging/tracing).
func (p *planner) ctx() context.Context {
	return p.session.Ctx()
}

// setTxn implements the queryRunner interface.
func (p *planner) setTxn(txn *client.Txn) {
	p.txn = txn
	if txn != nil {
		p.evalCtx.SetClusterTimestamp(txn.Proto.OrigTimestamp)
	} else {
		p.evalCtx.SetTxnTimestamp(time.Time{})
		p.evalCtx.SetStmtTimestamp(time.Time{})
		p.evalCtx.SetClusterTimestamp(hlc.ZeroTimestamp)
	}
}

// resetTxn implements the queryRunner interface.
func (p *planner) resetTxn() {
	p.setTxn(nil)
}

// resetContexts (re-)initializes the structures
// needed for expression handling.
func (p *planner) resetContexts() {
	// Need to reset the parser because it cannot be reused between
	// batches.
	p.parser = parser.Parser{}

	p.semaCtx = parser.MakeSemaContext()
	p.semaCtx.Location = &p.session.Location

	p.evalCtx = parser.EvalContext{
		Location: &p.session.Location,
	}
}

// runShowTransactionState returns the state of current transaction.
func (p *planner) runShowTransactionState(txnState *txnState, implicitTxn bool) (Result, error) {
	var result Result
	result.PGTag = (*parser.Show)(nil).StatementTag()
	result.Type = (*parser.Show)(nil).StatementType()
	result.Columns = ResultColumns{{Name: "TRANSACTION STATUS", Typ: parser.TypeString}}
	result.Rows = NewRowContainer(p.session.makeBoundAccount(), result.Columns, 0)
	state := txnState.State
	if implicitTxn {
		state = NoTxn
	}
	if _, err := result.Rows.AddRow(parser.DTuple{parser.NewDString(state.String())}); err != nil {
		result.Rows.Close()
		result.Err = err
		return result, err
	}
	return result, nil
}

// noteworthyInternalMemoryUsageBytes is the minimum size tracked by
// each internal SQL pool before the pool start explicitly logging
// overall usage growth in the log.
var noteworthyInternalMemoryUsageBytes = envutil.EnvOrDefaultInt64("COCKROACH_NOTEWORTHY_INTERNAL_MEMORY_USAGE", 100*1024)

func makeInternalPlanner(
	opName string, txn *client.Txn, user string, memMetrics *MemoryMetrics,
) *planner {
	p := makePlanner(opName)
	p.setTxn(txn)
	p.resetContexts()
	p.session.User = user

	p.session.mon = mon.MakeUnlimitedMonitor(p.session.context,
		"internal-root",
		memMetrics.CurBytesCount, memMetrics.MaxBytesHist,
		noteworthyInternalMemoryUsageBytes)

	p.session.sessionMon = mon.MakeMonitor("internal-session",
		memMetrics.SessionCurBytesCount,
		memMetrics.SessionMaxBytesHist,
		-1, noteworthyInternalMemoryUsageBytes/5)
	p.session.sessionMon.Start(p.session.context, &p.session.mon, mon.BoundAccount{})

	p.session.TxnState.mon = mon.MakeMonitor("internal-txn",
		memMetrics.TxnCurBytesCount,
		memMetrics.TxnMaxBytesHist,
		-1, noteworthyInternalMemoryUsageBytes/5)
	p.session.TxnState.mon.Start(p.session.context, &p.session.mon, mon.BoundAccount{})

	return p
}

func finishInternalPlanner(p *planner) {
	p.session.TxnState.mon.Stop(p.session.context)
	p.session.sessionMon.Stop(p.session.context)
	p.session.mon.Stop(p.session.context)
}

// resetForBatch implements the queryRunner interface.
func (p *planner) resetForBatch(e *Executor) {
	// Update the systemConfig to a more recent copy, so that we can use tables
	// that we created in previus batches of the same transaction.
	cfg, cache := e.getSystemConfig()
	p.systemConfig = cfg
	p.databaseCache = cache
	p.session.TxnState.schemaChangers.curGroupNum++
	p.resetContexts()
	p.evalCtx.NodeID = e.cfg.NodeID.Get()
	p.evalCtx.ReCache = e.reCache
	p.evalCtx.Database = p.session.Database
	p.evalCtx.SearchPath = p.session.SearchPath
}

// query initializes a planNode from a SQL statement string. Close() must be
// called on the returned planNode after use.
func (p *planner) query(sql string, args ...interface{}) (planNode, error) {
	stmt, err := parser.ParseOneTraditional(sql)
	if err != nil {
		return nil, err
	}
	golangFillQueryArguments(p.semaCtx.Placeholders, args)
	return p.makePlan(stmt, false)
}

// queryRow implements the queryRunner interface.
func (p *planner) queryRow(sql string, args ...interface{}) (parser.DTuple, error) {
	plan, err := p.query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer plan.Close()
	if err := plan.Start(); err != nil {
		return nil, err
	}
	if next, err := plan.Next(); !next {
		return nil, err
	}
	values := plan.Values()
	next, err := plan.Next()
	if err != nil {
		return nil, err
	}
	if next {
		return nil, errors.Errorf("%s: unexpected multiple results", sql)
	}
	return values, nil
}

// exec implements the queryRunner interface.
func (p *planner) exec(sql string, args ...interface{}) (int, error) {
	plan, err := p.query(sql, args...)
	if err != nil {
		return 0, err
	}
	defer plan.Close()
	if err := plan.Start(); err != nil {
		return 0, err
	}
	return countRowsAffected(plan)
}

// setTestingVerifyMetadata implements the queryRunner interface.
func (p *planner) setTestingVerifyMetadata(fn func(config.SystemConfig) error) {
	p.testingVerifyMetadataFn = fn
	p.verifyFnCheckedOnce = false
}

// blockConfigUpdatesMaybe implements the queryRunner interface.
func (p *planner) blockConfigUpdatesMaybe(e *Executor) func() {
	if !e.cfg.TestingKnobs.WaitForGossipUpdate {
		return func() {}
	}
	return e.blockConfigUpdates()
}

// checkTestingVerifyMetadataInitialOrDie implements the queryRunner interface.
func (p *planner) checkTestingVerifyMetadataInitialOrDie(e *Executor, stmts parser.StatementList) {
	if !p.execCfg.TestingKnobs.WaitForGossipUpdate {
		return
	}
	// If there's nothinging to verify, or we've already verified the initial
	// condition, there's nothing to do.
	if p.testingVerifyMetadataFn == nil || p.verifyFnCheckedOnce {
		return
	}
	if p.testingVerifyMetadataFn(e.systemConfig) == nil {
		panic(fmt.Sprintf(
			"expected %q (or the statements before them) to require a "+
				"gossip update, but they did not", stmts))
	}
	p.verifyFnCheckedOnce = true
}

// checkTestingVerifyMetadataOrDie implements the queryRunner interface.
func (p *planner) checkTestingVerifyMetadataOrDie(e *Executor, stmts parser.StatementList) {
	if !p.execCfg.TestingKnobs.WaitForGossipUpdate ||
		p.testingVerifyMetadataFn == nil {
		return
	}
	if !p.verifyFnCheckedOnce {
		panic("initial state of the condition to verify was not checked")
	}

	for p.testingVerifyMetadataFn(e.systemConfig) != nil {
		e.waitForConfigUpdate()
	}
	p.testingVerifyMetadataFn = nil
}

func (p *planner) fillFKTableMap(m tableLookupsByID) error {
	for tableID := range m {
		table, err := p.getTableLeaseByID(tableID)
		if err == errTableAdding {
			m[tableID] = tableLookup{isAdding: true}
			continue
		}
		if err != nil {
			return err
		}
		m[tableID] = tableLookup{table: table}
	}
	return nil
}
