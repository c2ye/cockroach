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

package sqlmigrations

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/internal/client"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlbase"
	"github.com/cockroachdb/cockroach/pkg/util/envutil"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/cockroachdb/cockroach/pkg/util/retry"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
)

var (
	leaseDuration        = time.Minute
	leaseRefreshInterval = leaseDuration / 5
)

// MigrationManagerTestingKnobs contains testing knobs.
type MigrationManagerTestingKnobs struct {
	// DisableMigrations skips all migrations.
	DisableMigrations bool
	// DisableBackfillMigrations stops applying migrations once
	// a migration with 'doesBackfill == true' is encountered.
	// TODO(mberhault): we could skip only backfill migrations and dependencies
	// if we had some concept of migration dependencies.
	DisableBackfillMigrations bool
}

// ModuleTestingKnobs is part of the base.ModuleTestingKnobs interface.
func (*MigrationManagerTestingKnobs) ModuleTestingKnobs() {}

var _ base.ModuleTestingKnobs = &MigrationManagerTestingKnobs{}

// backwardCompatibleMigrations is a hard-coded list of migrations to be run on
// startup. They will always be run from top-to-bottom, and because they are
// assumed to be backward-compatible, they will be run regardless of what other
// node versions are currently running within the cluster.
// Migrations must be idempotent: a migration may run successfully but not be recorded
// as completed, causing a second run.
var backwardCompatibleMigrations = []migrationDescriptor{
	{
		// Introduced in v1.0. Baked into v2.0.
		name: "default UniqueID to uuid_v4 in system.eventlog",
	},
	{
		// Introduced in v1.0. Baked into v2.0.
		name: "create system.jobs table",
	},
	{
		// Introduced in v1.0. Baked into v2.0.
		name: "create system.settings table",
	},
	{
		// Introduced in v1.0. Permanent migration.
		name:   "enable diagnostics reporting",
		workFn: optInToDiagnosticsStatReporting,
	},
	{
		// Introduced in v1.1. Baked into v2.0.
		name: "establish conservative dependencies for views #17280 #17269 #17306",
	},
	{
		// Introduced in v1.1. Baked into v2.0.
		name: "create system.sessions table",
	},
	{
		// Introduced in v1.1. Permanent migration.
		name:   "populate initial version cluster setting table entry",
		workFn: populateVersionSetting,
	},
	{
		// Introduced in v1.1. Permanent migration.
		name:   "persist trace.debug.enable = 'false'",
		workFn: disableNetTrace,
	},
	{
		// Introduced in v2.0. Baked into v2.1.
		name: "create system.table_statistics table",
	},
	{
		// Introduced in v2.0. Permanent migration.
		name:   "add root user",
		workFn: addRootUser,
	},
	{
		// Introduced in v2.0. Baked into v2.1.
		name: "create system.locations table",
	},
	{
		// Introduced in v2.0. Baked into v2.1.
		name: "add default .meta and .liveness zone configs",
	},
	{
		// Introduced in v2.0. Baked into v2.1.
		name: "create system.role_members table",
	},
	{
		// Introduced in v2.0. Permanent migration.
		name:   "add system.users isRole column and create admin role",
		workFn: addAdminRole,
	},
	{
		// Introduced in v2.0, replaced by "ensure admin role privileges in all descriptors"
		name: "grant superuser privileges on all objects to the admin role",
	},
	{
		// Introduced in v2.0. Permanent migration.
		name:   "make root a member of the admin role",
		workFn: addRootToAdminRole,
	},
	{
		// Introduced in v2.0. Baked into v2.1.
		name: "upgrade table descs to interleaved format version",
	},
	{
		// Introduced in v2.0 alphas then folded into `retiredSettings`.
		name: "remove cluster setting `kv.gc.batch_size`",
	},
	{
		// Introduced in v2.0 alphas then folded into `retiredSettings`.
		name: "remove cluster setting `kv.transaction.max_intents`",
	},
	{
		// Introduced in v2.0. Baked into v2.1.
		name: "add default system.jobs zone config",
	},
	{
		// Introduced in v2.0. Permanent migration.
		name:   "initialize cluster.secret",
		workFn: initializeClusterSecret,
	},
	{
		// Introduced in v2.0. Repeated in v2.1 below.
		name: "ensure admin role privileges in all descriptors",
	},
	{
		// Introduced in v2.1, repeat of 2.0 migration to catch mixed-version issues.
		// TODO(mberhault): bake into v2.2.
		name:   "repeat: ensure admin role privileges in all descriptors",
		workFn: ensureMaxPrivileges,
	},
	{
		// Introduced in v2.1.
		// TODO(mberhault): bake into v2.2.
		name:   "disallow public user or role name",
		workFn: disallowPublicUserOrRole,
	},
}

// migrationDescriptor describes a single migration hook that's used to modify
// some part of the cluster state when the CockroachDB version is upgraded.
// See docs/RFCs/cluster_upgrade_tool.md for details.
type migrationDescriptor struct {
	// name must be unique amongst all hard-coded migrations.
	name string
	// workFn must be idempotent so that we can safely re-run it if a node failed
	// while running it.
	workFn func(context.Context, runner) error
	// doesBackfill should be set to true if the migration triggers a backfill.
	doesBackfill bool
	// newDescriptorIDs lists the IDs of any additional descriptors added by this
	// migration. This is needed to automate certain tests, which check the number
	// of ranges/descriptors present on server bootup.
	newDescriptorIDs sqlbase.IDs
}

func init() {
	// Ensure that all migrations have unique names.
	names := make(map[string]struct{}, len(backwardCompatibleMigrations))
	for _, migration := range backwardCompatibleMigrations {
		name := migration.name
		if _, ok := names[name]; ok {
			log.Fatalf(context.Background(), "duplicate sql migration %q", name)
		}
		names[name] = struct{}{}
	}
}

type runner struct {
	db          db
	sqlExecutor *sql.Executor
	memMetrics  *sql.MemoryMetrics
}

func (r *runner) newRootSession(ctx context.Context) *sql.Session {
	args := sql.SessionArgs{User: security.NodeUser}
	s := sql.NewSession(ctx, args, r.sqlExecutor, r.memMetrics, nil /* conn */)
	s.StartUnlimitedMonitor()
	return s
}

// leaseManager is defined just to allow us to use a fake client.LeaseManager
// when testing this package.
type leaseManager interface {
	AcquireLease(ctx context.Context, key roachpb.Key) (*client.Lease, error)
	ExtendLease(ctx context.Context, l *client.Lease) error
	ReleaseLease(ctx context.Context, l *client.Lease) error
	TimeRemaining(l *client.Lease) time.Duration
}

// db is defined just to allow us to use a fake client.DB when testing this
// package.
type db interface {
	Scan(ctx context.Context, begin, end interface{}, maxRows int64) ([]client.KeyValue, error)
	Put(ctx context.Context, key, value interface{}) error
	Txn(ctx context.Context, retryable func(ctx context.Context, txn *client.Txn) error) error
}

// Manager encapsulates the necessary functionality for handling migrations
// of data in the cluster.
type Manager struct {
	stopper      *stop.Stopper
	leaseManager leaseManager
	db           db
	sqlExecutor  *sql.Executor
	memMetrics   *sql.MemoryMetrics
	testingKnobs MigrationManagerTestingKnobs
}

// NewManager initializes and returns a new Manager object.
func NewManager(
	stopper *stop.Stopper,
	db *client.DB,
	executor *sql.Executor,
	clock *hlc.Clock,
	testingKnobs MigrationManagerTestingKnobs,
	memMetrics *sql.MemoryMetrics,
	clientID string,
) *Manager {
	opts := client.LeaseManagerOptions{
		ClientID:      clientID,
		LeaseDuration: leaseDuration,
	}
	return &Manager{
		stopper:      stopper,
		leaseManager: client.NewLeaseManager(db, clock, opts),
		db:           db,
		sqlExecutor:  executor,
		memMetrics:   memMetrics,
		testingKnobs: testingKnobs,
	}
}

// ExpectedDescriptorIDs returns the list of all expected system descriptor IDs,
// including those added by completed migrations. This is needed for certain
// tests, which check the number of ranges and system tables at node startup.
//
// NOTE: This value may be out-of-date if another node is actively running
// migrations, and so should only be used in test code where the migration
// lifecycle is tightly controlled.
func ExpectedDescriptorIDs(ctx context.Context, db db) (sqlbase.IDs, error) {
	completedMigrations, err := getCompletedMigrations(ctx, db)
	if err != nil {
		return nil, err
	}
	descriptorIDs := sqlbase.MakeMetadataSchema().DescriptorIDs()
	for _, migration := range backwardCompatibleMigrations {
		if _, ok := completedMigrations[string(migrationKey(migration))]; ok {
			descriptorIDs = append(descriptorIDs, migration.newDescriptorIDs...)
		}
	}
	sort.Sort(descriptorIDs)
	return descriptorIDs, nil
}

// EnsureMigrations should be run during node startup to ensure that all
// required migrations have been run (and running all those that are definitely
// safe to run).
func (m *Manager) EnsureMigrations(ctx context.Context) error {
	if m.testingKnobs.DisableMigrations {
		log.Info(ctx, "skipping all migrations due to testing knob")
		return nil
	}

	// First, check whether there are any migrations that need to be run.
	completedMigrations, err := getCompletedMigrations(ctx, m.db)
	if err != nil {
		return err
	}
	allMigrationsCompleted := true
	for _, migration := range backwardCompatibleMigrations {
		if migration.workFn == nil {
			// Migration has been baked in. Ignore it.
			continue
		}
		if m.testingKnobs.DisableBackfillMigrations && migration.doesBackfill {
			log.Infof(ctx, "ignoring migrations after (and including) %s due to testing knob",
				migration.name)
			break
		}
		key := migrationKey(migration)
		if _, ok := completedMigrations[string(key)]; !ok {
			allMigrationsCompleted = false
		}
	}
	if allMigrationsCompleted {
		return nil
	}

	// If there are any, grab the migration lease to ensure that only one
	// node is ever doing migrations at a time.
	// Note that we shouldn't ever let client.LeaseNotAvailableErrors cause us
	// to stop trying, because if we return an error the server will be shut down,
	// and this server being down may prevent the leaseholder from finishing.
	var lease *client.Lease
	if log.V(1) {
		log.Info(ctx, "trying to acquire lease")
	}
	for r := retry.StartWithCtx(ctx, base.DefaultRetryOptions()); r.Next(); {
		lease, err = m.leaseManager.AcquireLease(ctx, keys.MigrationLease)
		if err == nil {
			break
		}
		log.Errorf(ctx, "failed attempt to acquire migration lease: %s", err)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to acquire lease for running necessary migrations")
	}

	// Ensure that we hold the lease throughout the migration process and release
	// it when we're done.
	done := make(chan interface{}, 1)
	defer func() {
		done <- nil
		if log.V(1) {
			log.Info(ctx, "trying to release the lease")
		}
		if err := m.leaseManager.ReleaseLease(ctx, lease); err != nil {
			log.Errorf(ctx, "failed to release migration lease: %s", err)
		}
	}()
	if err := m.stopper.RunAsyncTask(ctx, "migrations.Manager: lease watcher",
		func(ctx context.Context) {
			select {
			case <-done:
				return
			case <-time.After(leaseRefreshInterval):
				if err := m.leaseManager.ExtendLease(ctx, lease); err != nil {
					log.Warningf(ctx, "unable to extend ownership of expiration lease: %s", err)
				}
				if m.leaseManager.TimeRemaining(lease) < leaseRefreshInterval {
					// Do one last final check of whether we're done - it's possible that
					// ReleaseLease can sneak in and execute ahead of ExtendLease even if
					// the ExtendLease started first (making for an unexpected value error),
					// and doing this final check can avoid unintended shutdowns.
					select {
					case <-done:
						return
					default:
						// Note that we may be able to do better than this by influencing the
						// deadline of migrations' transactions based on the lease expiration
						// time, but simply kill the process for now for the sake of simplicity.
						log.Fatal(ctx, "not enough time left on migration lease, terminating for safety")
					}
				}
			}
		}); err != nil {
		return err
	}

	// Re-get the list of migrations in case any of them were completed between
	// our initial check and our grabbing of the lease.
	completedMigrations, err = getCompletedMigrations(ctx, m.db)
	if err != nil {
		return err
	}

	startTime := timeutil.Now().String()
	r := runner{
		db:          m.db,
		sqlExecutor: m.sqlExecutor,
		memMetrics:  m.memMetrics,
	}
	for _, migration := range backwardCompatibleMigrations {
		if migration.workFn == nil {
			// Migration has been baked in. Ignore it.
			continue
		}

		key := migrationKey(migration)
		if _, ok := completedMigrations[string(key)]; ok {
			continue
		}

		if m.testingKnobs.DisableBackfillMigrations && migration.doesBackfill {
			log.Infof(ctx, "ignoring migrations after (and including) %s due to testing knob",
				migration.name)
			break
		}

		if log.V(1) {
			log.Infof(ctx, "running migration %q", migration.name)
		}
		if err := migration.workFn(ctx, r); err != nil {
			return errors.Wrapf(err, "failed to run migration %q", migration.name)
		}

		if log.V(1) {
			log.Infof(ctx, "trying to persist record of completing migration %s", migration.name)
		}
		if err := m.db.Put(ctx, key, startTime); err != nil {
			return errors.Wrapf(err, "failed to persist record of completing migration %q",
				migration.name)
		}
	}

	return nil
}

func getCompletedMigrations(ctx context.Context, db db) (map[string]struct{}, error) {
	if log.V(1) {
		log.Info(ctx, "trying to get the list of completed migrations")
	}
	keyvals, err := db.Scan(ctx, keys.MigrationPrefix, keys.MigrationKeyMax, 0 /* maxRows */)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get list of completed migrations")
	}
	completedMigrations := make(map[string]struct{})
	for _, keyval := range keyvals {
		completedMigrations[string(keyval.Key)] = struct{}{}
	}
	return completedMigrations, nil
}

func migrationKey(migration migrationDescriptor) roachpb.Key {
	return append(keys.MigrationPrefix, roachpb.RKey(migration.name)...)
}

func createSystemTable(ctx context.Context, r runner, desc sqlbase.TableDescriptor) error {
	// We install the table at the KV layer so that we can choose a known ID in
	// the reserved ID space. (The SQL layer doesn't allow this.)
	err := r.db.Txn(ctx, func(ctx context.Context, txn *client.Txn) error {
		b := txn.NewBatch()
		b.CPut(sqlbase.MakeNameMetadataKey(desc.GetParentID(), desc.GetName()), desc.GetID(), nil)
		b.CPut(sqlbase.MakeDescMetadataKey(desc.GetID()), sqlbase.WrapDescriptor(&desc), nil)
		if err := txn.SetSystemConfigTrigger(); err != nil {
			return err
		}
		return txn.Run(ctx, b)
	})
	if err != nil {
		// CPuts only provide idempotent inserts if we ignore the errors that arise
		// when the condition isn't met.
		if _, ok := err.(*roachpb.ConditionFailedError); ok {
			return nil
		}
	}
	return err
}

var reportingOptOut = envutil.EnvOrDefaultBool("COCKROACH_SKIP_ENABLING_DIAGNOSTIC_REPORTING", false)

func runStmtAsRootWithRetry(
	ctx context.Context, r runner, stmt string, pinfo *tree.PlaceholderInfo,
) error {
	// System tables can only be modified by a privileged internal user.
	session := r.newRootSession(ctx)
	defer session.Finish(r.sqlExecutor)
	// Retry a limited number of times because returning an error and letting
	// the node kill itself is better than holding the migration lease for an
	// arbitrarily long time.
	var err error
	for retry := retry.Start(retry.Options{MaxRetries: 5}); retry.Next(); {
		var res sql.StatementResults
		res, err = r.sqlExecutor.ExecuteStatementsBuffered(session, stmt, pinfo, 1)
		if err == nil {
			res.Close(ctx)
			break
		}
		log.Warningf(ctx, "failed to run %s: %v", stmt, err)
	}
	return err
}

// SettingsDefaultOverrides documents the effect of several migrations that add
// an explicit value for a setting, effectively changing the "default value"
// from what was defined in code.
var SettingsDefaultOverrides = map[string]string{
	"diagnostics.reporting.enabled": "true",
	"trace.debug.enable":            "false",
	"cluster.secret":                "<random>",
}

func optInToDiagnosticsStatReporting(ctx context.Context, r runner) error {
	// We're opting-out of the automatic opt-in. See discussion in updates.go.
	if reportingOptOut {
		return nil
	}
	return runStmtAsRootWithRetry(ctx, r, `SET CLUSTER SETTING diagnostics.reporting.enabled = true`, nil)
}

func disableNetTrace(ctx context.Context, r runner) error {
	return runStmtAsRootWithRetry(ctx, r, `SET CLUSTER SETTING trace.debug.enable = false`, nil)
}

func initializeClusterSecret(ctx context.Context, r runner) error {
	return runStmtAsRootWithRetry(ctx, r, `SET CLUSTER SETTING cluster.secret = gen_random_uuid()::STRING`, nil)
}

func populateVersionSetting(ctx context.Context, r runner) error {
	var v roachpb.Version
	if err := r.db.Txn(ctx, func(ctx context.Context, txn *client.Txn) error {
		return txn.GetProto(ctx, keys.BootstrapVersionKey, &v)
	}); err != nil {
		return err
	}
	if v == (roachpb.Version{}) {
		// The cluster was bootstrapped at v1.0 (or even earlier), so make that
		// the version.
		v = cluster.VersionByKey(cluster.VersionBase)
	}

	b, err := protoutil.Marshal(&cluster.ClusterVersion{
		MinimumVersion: v,
		UseVersion:     v,
	})
	if err != nil {
		return errors.Wrap(err, "while marshaling version")
	}

	// System tables can only be modified by a privileged internal user.
	session := r.newRootSession(ctx)
	defer session.Finish(r.sqlExecutor)

	// Add a ON CONFLICT DO NOTHING to avoid changing an existing version.
	// Again, this can happen if the migration doesn't run to completion
	// (overwriting also seems reasonable, but what for).
	// We don't allow users to perform version changes until we have run
	// the insert below.
	if res, err := r.sqlExecutor.ExecuteStatementsBuffered(
		session,
		fmt.Sprintf(`INSERT INTO system.settings (name, value, "lastUpdated", "valueType") VALUES ('version', x'%x', NOW(), 'm') ON CONFLICT(name) DO NOTHING`, b),
		nil, 1,
	); err == nil {
		res.Close(ctx)
	} else if err != nil {
		return err
	}

	pl := tree.MakePlaceholderInfo()
	pl.SetValue("1", tree.NewDString(v.String()))
	if res, err := r.sqlExecutor.ExecuteStatementsBuffered(
		session, "SET CLUSTER SETTING version = $1", &pl, 1); err == nil {
		res.Close(ctx)
	} else if err != nil {
		return err
	}
	return nil
}

func addRootUser(ctx context.Context, r runner) error {
	// Upsert the root user into the table. We intentionally override any existing entry.
	const upsertRootStmt = `
	        UPSERT INTO system.users (username, "hashedPassword", "isRole") VALUES ($1, '', false)
	        `

	pl := tree.MakePlaceholderInfo()
	pl.SetValue("1", tree.NewDString(security.RootUser))
	return runStmtAsRootWithRetry(ctx, r, upsertRootStmt, &pl)
}

func addAdminRole(ctx context.Context, r runner) error {
	// Upsert the admin role into the table. We intentionally override any existing entry.
	const upsertAdminStmt = `
          UPSERT INTO system.users (username, "hashedPassword", "isRole") VALUES ($1, '', true)
          `

	pl := tree.MakePlaceholderInfo()
	pl.SetValue("1", tree.NewDString(sqlbase.AdminRole))
	return runStmtAsRootWithRetry(ctx, r, upsertAdminStmt, &pl)
}

func addRootToAdminRole(ctx context.Context, r runner) error {
	// Upsert the role membership into the table. We intentionally override any existing entry.
	const upsertAdminStmt = `
          UPSERT INTO system.role_members ("role", "member", "isAdmin") VALUES ($1, $2, true)
          `

	pl := tree.MakePlaceholderInfo()
	pl.SetValue("1", tree.NewDString(sqlbase.AdminRole))
	pl.SetValue("2", tree.NewDString(security.RootUser))
	return runStmtAsRootWithRetry(ctx, r, upsertAdminStmt, &pl)
}

// ensureMaxPrivileges ensures that all descriptors have privileges
// for the admin role, and that root and user privileges do not exceed
// the allowed privileges.
//
// TODO(mberhault): Remove this migration in v2.1.
func ensureMaxPrivileges(ctx context.Context, r runner) error {
	tableDescFn := func(desc *sqlbase.TableDescriptor) (bool, error) {
		return desc.Privileges.MaybeFixPrivileges(desc.ID), nil
	}
	databaseDescFn := func(desc *sqlbase.DatabaseDescriptor) (bool, error) {
		return desc.Privileges.MaybeFixPrivileges(desc.ID), nil
	}
	return upgradeDescsWithFn(ctx, r, tableDescFn, databaseDescFn)
}

var upgradeDescBatchSize int64 = 50

// upgradeTableDescsWithFn runs the provided upgrade functions on each table
// and database descriptor, persisting any upgrades if the function indicates that the
// descriptor was changed.
// Upgrade functions may be nil to perform nothing for the corresponding descriptor type.
func upgradeDescsWithFn(
	ctx context.Context,
	r runner,
	upgradeTableDescFn func(desc *sqlbase.TableDescriptor) (upgraded bool, err error),
	upgradeDatabaseDescFn func(desc *sqlbase.DatabaseDescriptor) (upgraded bool, err error),
) error {
	session := r.newRootSession(ctx)
	defer session.Finish(r.sqlExecutor)

	startKey := sqlbase.MakeAllDescsMetadataKey()
	endKey := startKey.PrefixEnd()
	for done := false; !done; {
		// It's safe to use multiple transactions here. Any table descriptor that's
		// created while this migration is in progress will use the desired
		// InterleavedFormatVersion, as all possible binary versions in the cluster
		// (the current release and the previous release) create new table
		// descriptors using InterleavedFormatVersion. We need only upgrade the
		// ancient table descriptors written by versions before beta-20161013.
		if err := r.db.Txn(ctx, func(ctx context.Context, txn *client.Txn) error {
			kvs, err := txn.Scan(ctx, startKey, endKey, upgradeDescBatchSize)
			if err != nil {
				return err
			}
			if len(kvs) == 0 {
				done = true
				return nil
			}
			startKey = kvs[len(kvs)-1].Key.Next()

			b := txn.NewBatch()
			for _, kv := range kvs {
				var sqlDesc sqlbase.Descriptor
				if err := kv.ValueProto(&sqlDesc); err != nil {
					return err
				}
				switch t := sqlDesc.Union.(type) {
				case *sqlbase.Descriptor_Table:
					if table := sqlDesc.GetTable(); table != nil && upgradeTableDescFn != nil {
						if upgraded, err := upgradeTableDescFn(table); err != nil {
							return err
						} else if upgraded {
							// Though SetUpVersion typically bails out if the table is being
							// dropped, it's safe to ignore the DROP state here and
							// unconditionally set UpVersion. For proof, see
							// TestDropTableWhileUpgradingFormat.
							//
							// In fact, it's of the utmost importance that this migration
							// upgrades every last old-format table descriptor, including those
							// that are dropping. Otherwise, the user could upgrade to a version
							// without support for reading the old format before the drop
							// completes, leaving a broken table descriptor and the table's
							// remaining data around forever. This isn't just a theoretical
							// concern: consider that dropping a large table can take several
							// days, while upgrading to a new version can take as little as a
							// few minutes.
							table.UpVersion = true
							if err := table.Validate(ctx, txn, nil); err != nil {
								return err
							}

							b.Put(kv.Key, sqlbase.WrapDescriptor(table))
						}
					}
				case *sqlbase.Descriptor_Database:
					if database := sqlDesc.GetDatabase(); database != nil && upgradeDatabaseDescFn != nil {
						if upgraded, err := upgradeDatabaseDescFn(database); err != nil {
							return err
						} else if upgraded {
							if err := database.Validate(); err != nil {
								return err
							}

							b.Put(kv.Key, sqlbase.WrapDescriptor(database))
						}
					}

				default:
					return errors.Errorf("Descriptor.Union has unexpected type %T", t)
				}
			}
			if err := txn.SetSystemConfigTrigger(); err != nil {
				return err
			}
			return txn.Run(ctx, b)
		}); err != nil {
			return err
		}
	}
	return nil
}

func disallowPublicUserOrRole(ctx context.Context, r runner) error {
	// System tables can only be modified by a privileged internal user.
	session := r.newRootSession(ctx)
	defer session.Finish(r.sqlExecutor)

	// Check whether a user or role named "public" exists.
	const selectPublicStmt = `
          SELECT username, "isRole" from system.users WHERE username = $1
          `

	pl := tree.MakePlaceholderInfo()
	pl.SetValue("1", tree.NewDString(sqlbase.PublicRole))

	for retry := retry.Start(retry.Options{MaxRetries: 5}); retry.Next(); {
		res, err := r.sqlExecutor.ExecuteStatementsBuffered(session, selectPublicStmt, &pl, 1)
		if err != nil {
			continue
		}
		defer res.Close(ctx)

		if len(res.ResultList) == 0 || res.ResultList[0].Rows.Len() == 0 {
			// No such user.
			return nil
		}

		if res.ResultList[0].Rows.Len() != 1 {
			log.Fatalf(context.Background(), "more than one system.users entry for username=%s: %v",
				sqlbase.PublicRole, res.ResultList)
		}

		firstRow := res.ResultList[0].Rows.At(0)

		isRole, ok := tree.AsDBool(firstRow[1])
		if !ok {
			log.Fatalf(context.Background(), "expected 'isRole' column of system.users to be of type bool, got %v",
				firstRow)
		}

		if isRole {
			return fmt.Errorf(`found a role named %s which is now a reserved name. Please drop the role `+
				`(DROP ROLE %s) using a previous version of CockroachDB and try again`,
				sqlbase.PublicRole, sqlbase.PublicRole)
		}
		return fmt.Errorf(`found a user named %s which is now a reserved name. Please drop the role `+
			`(DROP USER %s) using a previous version of CockroachDB and try again`,
			sqlbase.PublicRole, sqlbase.PublicRole)
	}
	return nil
}
