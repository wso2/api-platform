/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package runtime

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb"

	"github.com/wso2/api-platform/tests/framework/core/components"
)

// Credentials are one block's database credentials.
type Credentials struct {
	User     string
	Password string
}

// NewCredentials generates credentials for a block.
func NewCredentials() (Credentials, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return Credentials{}, fmt.Errorf("runtime: generating credentials: %w", err)
	}
	// Engine-safe alphabet: base64url minus characters that need quoting in a DSN or a
	// shell, plus a fixed prefix so every engine's password-complexity rule is met
	// (SQL Server rejects anything without mixed case, a digit and a symbol).
	pw := strings.NewReplacer("-", "x", "_", "y", "=", "z").
		Replace(base64.URLEncoding.EncodeToString(raw))
	return Credentials{User: "apip_it", Password: "Aa1!" + pw}, nil
}

// engine encapsulates everything engine-specific about provisioning.
//
// Keeping this behind one interface is what lets the block engine treat "provision the
// storage this block needs" as a single step regardless of which engines are involved,
// and what lets a new engine be added without touching the planner.
type engine interface {
	// serverDefinition builds the component definition for this engine's server
	// container, including whatever bootstrap the engine can do at first boot.
	serverDefinition(creds Credentials, stores []*Store, repoRoot string, image ...components.ImageRef) (*components.Definition, error)

	// canBootstrapAtStart reports whether serverDefinition's container creates the
	// databases and applies the schema by itself. When false, the caller must run
	// applySchema after the server is healthy.
	canBootstrapAtStart() bool

	// bootAttempts is how many times the server container may be started before its
	// boot failure is fatal. 1 means fail fast, which every engine without a KNOWN
	// probabilistic boot crash must keep — retrying hides real failures.
	bootAttempts() int

	// dsn builds the coordinates one component uses to reach a store.
	dsn(store *Store, alias string, creds Credentials) components.DSN

	// applySchema reaches PhaseSchemaApplied by connecting to a running server. Called
	// only when canBootstrapAtStart reports false; an engine that bootstraps at first
	// boot implements this as a no-op.
	applySchema(
		ctx context.Context, server *Container, creds Credentials,
		stores []*Store, repoRoot string,
	) error
}

// engineFor returns the engine implementation for a type.
func engineFor(t components.DBType) (engine, error) {
	switch t {
	case components.Postgres:
		return postgresEngine{}, nil
	case components.SQLServer:
		return sqlServerEngine{}, nil
	case components.SQLite:
		return nil, fmt.Errorf("runtime: %q is embedded and has no server engine", t)
	default:
		return nil, fmt.Errorf("runtime: no engine for %q", t)
	}
}

// postgresEngine provisions PostgreSQL.
//
// Postgres can do the whole bootstrap at first boot: anything in its init directory runs
// once, before the server accepts external connections. So databases and schema are
// applied from a generated script rather than by connecting afterwards — which is also
// how the suites this replaces did it, and it keeps "server healthy" and "schema
// applied" from racing.
type postgresEngine struct{}

const (
	postgresAlias = "it-postgres"
	postgresPort  = 5432
)

func (postgresEngine) canBootstrapAtStart() bool { return true }

func (postgresEngine) bootAttempts() int { return 1 }

func (e postgresEngine) serverDefinition(
	creds Credentials, stores []*Store, repoRoot string, image ...components.ImageRef,
) (*components.Definition, error) {
	script, err := e.bootstrapScript(stores, repoRoot)
	if err != nil {
		return nil, err
	}
	scriptPath, err := writeTempFile("postgres-init-*.sql", script)
	if err != nil {
		return nil, err
	}

	serverImage := components.ImageRef{Ref: "postgres:16-alpine"}
	if len(image) > 0 && image[0].Ref != "" {
		serverImage = image[0]
	}
	return &components.Definition{
		Name:  "postgres",
		Image: serverImage,
		Alias: postgresAlias,
		Endpoints: []components.Endpoint{
			{Name: "sql", Port: postgresPort, Scheme: "tcp", AwaitListening: true},
		},
		Env: map[string]string{
			"POSTGRES_USER":     creds.User,
			"POSTGRES_PASSWORD": creds.Password,
			// A neutral bootstrap runtime: every store gets its own, created by the
			// script, so nothing lands in a database named after one component.
			"POSTGRES_DB": "bootstrap",
		},
		Files: []components.FileMount{
			{
				HostPath:      scriptPath,
				ContainerPath: "/docker-entrypoint-initdb.d/00-framework-init.sql",
				Mode:          0o644,
			},
		},
		Limits: components.ResourceLimits{CPUs: 1, MemoryMB: 512},
	}, nil
}

// bootstrapScript generates the CREATE DATABASE and schema-application script.
//
// This is the generated equivalent of the hand-maintained init script the current
// suites carry. Generating it is the point: the hand-written version encodes database
// names and component pairings by convention, and drifts silently when a component is
// added.
func (postgresEngine) bootstrapScript(stores []*Store, repoRoot string) ([]byte, error) {
	var b strings.Builder
	b.WriteString("-- Generated by the integration-v2 framework. Do not edit.\n")
	b.WriteString("-- Components keep separate schemas with overlapping table names, so each\n")
	b.WriteString("-- gets its own database on the shared server.\n\n")

	for _, s := range stores {
		if s.Embedded {
			continue
		}
		fmt.Fprintf(&b, "CREATE DATABASE %s;  -- owner: %s\n", s.Database, s.Owner)
	}

	for _, s := range stores {
		if s.Embedded || len(s.Schema) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n\\connect %s\n", s.Database)
		for _, ddl := range s.Schema {
			path := ddl
			if !filepath.IsAbs(path) {
				path = filepath.Join(repoRoot, ddl)
			}
			content, err := os.ReadFile(path)
			if err != nil {
				// A missing DDL file must fail here, loudly, naming the component.
				// Provisioning an empty database instead would surface much later as
				// an inscrutable "relation does not exist" from the product.
				return nil, fmt.Errorf("runtime: reading schema %q for store %q (owner %s): %w",
					ddl, s.Database, s.Owner, err)
			}
			fmt.Fprintf(&b, "-- from %s\n%s\n", ddl, content)
		}
	}

	return []byte(b.String()), nil
}

// applySchema is a no-op: Postgres already applied everything at first boot, and
// re-running the script against the live server would double-apply it.
func (postgresEngine) applySchema(
	context.Context, *Container, Credentials, []*Store, string,
) error {
	return nil
}

func (postgresEngine) dsn(store *Store, alias string, creds Credentials) components.DSN {
	return components.DSN{
		Type:     components.Postgres,
		Host:     alias,
		Port:     postgresPort,
		Database: store.Database,
		User:     creds.User,
		Password: creds.Password,
		SSLMode:  "disable",
	}
}

// writeTempFile stages generated content on the host so it can be copied into a
// container. Removed when the process exits; the block's teardown does not need to know
// about it.
func writeTempFile(pattern string, content []byte) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("runtime: staging generated file: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(content); err != nil {
		return "", fmt.Errorf("runtime: writing generated file %q: %w", f.Name(), err)
	}
	if err := os.Chmod(f.Name(), 0o644); err != nil {
		return "", fmt.Errorf("runtime: setting mode on %q: %w", f.Name(), err)
	}
	return f.Name(), nil
}

// sqlServerEngine provisions Microsoft SQL Server.
type sqlServerEngine struct{}

const (
	sqlServerAlias = "it-sqlserver"
	sqlServerPort  = 1433

	// sqlServerLoginTimeout bounds the wait for a successful administrative login.
	sqlServerLoginTimeout = 4 * time.Minute

	// sqlServerBootAttempts bounds retries for transient server startup failures.
	sqlServerBootAttempts = 3
)

func (sqlServerEngine) canBootstrapAtStart() bool { return false }

func (sqlServerEngine) bootAttempts() int { return sqlServerBootAttempts }

func (sqlServerEngine) serverDefinition(
	creds Credentials, stores []*Store, repoRoot string, image ...components.ImageRef,
) (*components.Definition, error) {
	// Validate every DDL path up front. A missing file must fail before the container
	// starts, naming the component — provisioning an empty database instead surfaces
	// much later as an inscrutable "invalid object name" from the product.
	for _, s := range stores {
		for _, ddl := range s.Schema {
			if _, err := os.Stat(resolveDDL(repoRoot, ddl)); err != nil {
				return nil, fmt.Errorf("runtime: reading schema %q for store %q (owner %s): %w",
					ddl, s.Database, s.Owner, err)
			}
		}
	}

	serverImage := components.ImageRef{
		Ref:    "mcr.microsoft.com/mssql/server:2022-latest",
		ByArch: map[string]string{"arm64": "mcr.microsoft.com/azure-sql-edge:latest"},
	}
	if len(image) > 0 && image[0].Ref != "" {
		serverImage = image[0]
	}
	return &components.Definition{
		Name:  "sqlserver",
		Image: serverImage,
		Alias: sqlServerAlias,
		Endpoints: []components.Endpoint{
			{Name: "sql", Port: sqlServerPort, Scheme: "tcp", AwaitListening: true},
		},
		Env: map[string]string{
			"ACCEPT_EULA": "Y",
			// The images use different environment variable names for the SA password.
			"MSSQL_SA_PASSWORD": creds.Password,
			"SA_PASSWORD":       creds.Password,
			"MSSQL_PID":         "Developer",
		},
		Limits: components.ResourceLimits{CPUs: 1, MemoryMB: 2048},
	}, nil
}

func (sqlServerEngine) dsn(store *Store, alias string, creds Credentials) components.DSN {
	return components.DSN{
		Type:     components.SQLServer,
		Host:     alias,
		Port:     sqlServerPort,
		Database: store.Database,
		// Components connect as sa because the suites do not exercise least-privilege
		// database users; that is a product concern, not a topology one.
		User:     "sa",
		Password: creds.Password,
	}
}

// applySchema reaches PhaseSchemaApplied by connecting over the mapped host port.
func (e sqlServerEngine) applySchema(
	ctx context.Context, server *Container, creds Credentials,
	stores []*Store, repoRoot string,
) error {
	port, err := server.Instance.MappedPort("sql")
	if err != nil {
		return fmt.Errorf("runtime: resolving the sqlserver port: %w", err)
	}
	// From the INSTANCE, never a hardcoded loopback: with colima's port forwarder disabled the
	// mapped port is reachable only at the VM's address, and 127.0.0.1 refuses instantly.
	host := server.Instance.Host()

	admin, err := e.awaitLogin(ctx, host, port, creds)
	if err != nil {
		return err
	}
	defer func() { _ = admin.Close() }()

	for _, s := range stores {
		if s.Embedded {
			continue
		}
		// Idempotent, so a retried provision does not fail on an existing runtime.
		create := fmt.Sprintf("IF DB_ID('%s') IS NULL CREATE DATABASE [%s]", s.Database, s.Database)
		if _, err := admin.ExecContext(ctx, create); err != nil {
			return fmt.Errorf("runtime: creating sqlserver database %q (owner %s): %w",
				s.Database, s.Owner, err)
		}
	}

	// A separate connection per runtime: SQL Server has no USE across a pooled
	// connection that can be relied on, and the driver selects the database at dial time.
	for _, s := range stores {
		if s.Embedded || len(s.Schema) == 0 {
			continue
		}
		if err := e.applyStoreSchema(ctx, host, port, creds, s, repoRoot); err != nil {
			return err
		}
	}
	return nil
}

func (e sqlServerEngine) applyStoreSchema(
	ctx context.Context, host string, port int, creds Credentials, s *Store, repoRoot string,
) error {
	db, err := sql.Open("sqlserver", sqlServerDSN(host, port, creds, s.Database))
	if err != nil {
		return fmt.Errorf("runtime: opening %q: %w", s.Database, err)
	}
	defer func() { _ = db.Close() }()

	for _, ddl := range s.Schema {
		content, err := os.ReadFile(resolveDDL(repoRoot, ddl))
		if err != nil {
			return fmt.Errorf("runtime: reading schema %q for %q (owner %s): %w",
				ddl, s.Database, s.Owner, err)
		}
		for i, batch := range splitBatches(string(content)) {
			if _, err := db.ExecContext(ctx, batch); err != nil {
				return fmt.Errorf("runtime: applying schema %q batch %d to %q (owner %s): %w",
					ddl, i+1, s.Database, s.Owner, err)
			}
		}
	}
	return nil
}

// awaitLogin polls until the server accepts a login, returning an open master
// connection.
func (e sqlServerEngine) awaitLogin(ctx context.Context, host string, port int, creds Credentials) (*sql.DB, error) {
	if ctx == nil {
		return nil, fmt.Errorf("runtime: sqlserver login wait requires a context")
	}
	deadline := time.Now().Add(sqlServerLoginTimeout)
	attempts := 0
	var lastErr error

	for time.Now().Before(deadline) {
		attempts++
		db, err := sql.Open("sqlserver", sqlServerDSN(host, port, creds, "master"))
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err = db.PingContext(pingCtx)
			cancel()
			if err == nil {
				return db, nil
			}
			_ = db.Close()
		}
		lastErr = err

		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("runtime: sqlserver login wait cancelled after %d attempt(s): %w",
				attempts, ctxErr)
		}
		if !waitContext(ctx, 2*time.Second) {
			return nil, fmt.Errorf("runtime: sqlserver login wait cancelled after %d attempt(s): %w",
				attempts, ctx.Err())
		}
	}

	return nil, fmt.Errorf("runtime: sqlserver did not accept a login within %s (%d attempt(s)); "+
		"last error: %v", sqlServerLoginTimeout, attempts, lastErr)
}

// sqlServerDSN builds a driver DSN against the mapped host port.
func sqlServerDSN(host string, port int, creds Credentials, database string) string {
	u := &url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword("sa", creds.Password),
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
	}
	q := u.Query()
	q.Set("database", database)
	// The image generates a self-signed certificate. Asserting the harness trusts it
	// proves nothing about the product; certificate behaviour is a product assertion.
	q.Set("encrypt", "disable")
	u.RawQuery = q.Encode()
	return u.String()
}

// splitBatches splits a script on the GO separator that sqlcmd honours.
//
// A Go driver has no notion of GO — it would send the literal token to the server as a
// syntax error. No current DDL uses it, but a future one might, and the failure would be
// obscure.
func splitBatches(script string) []string {
	lines := strings.Split(script, "\n")
	var batches []string
	var current []string

	flush := func() {
		text := strings.TrimSpace(strings.Join(current, "\n"))
		if text != "" {
			batches = append(batches, text)
		}
		current = nil
	}

	for _, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), "GO") {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()

	return batches
}

// resolveDDL resolves a schema path against the repository root when it is relative.
func resolveDDL(repoRoot, ddl string) string {
	if filepath.IsAbs(ddl) || repoRoot == "" {
		return ddl
	}
	return filepath.Join(repoRoot, ddl)
}

// Request is one component's storage need within a block, as resolved by the topology
// loader: the definition, how many instances the block runs, and which engine was
// chosen for it.
type Request struct {
	Def      *components.Definition
	Type     components.DBType
	Image    components.ImageRef
	Replicas int
}

// StoreID identifies one logical database within a block.
type StoreID string

// Store is one logical database the framework will provision.
type Store struct {
	ID StoreID

	// Type is the engine backing this store.
	Type components.DBType

	// Database is the generated physical name. Generated, never authored: a
	// hand-written name collides the moment a matrix generates a block twice or two
	// components are assumed not to overlap and do.
	Database string

	// Owner is the component whose schema this store carries.
	Owner string

	// OwnerOrdinal distinguishes stores when the owner has replicas.
	OwnerOrdinal int

	// Schema is the DDL the framework must apply, in order. Empty when the owner
	// migrates itself or the engine needs no schema.
	Schema []string

	// Embedded is true when the engine lives inside the owning component's container
	// and so needs no server of its own.
	Embedded bool
}

// InstanceKey identifies one instance of one component within a block.
type InstanceKey string

// KeyFor builds the instance key for a component ordinal.
func KeyFor(component string, ordinal int) InstanceKey {
	return InstanceKey(fmt.Sprintf("%s#%d", component, ordinal))
}

// Server is one database container a block must run.
type Server struct {
	// Type is the engine.
	Type components.DBType
	// Image is the selected server image. An empty value uses the runtime default.
	Image components.ImageRef
	// Stores are the logical databases to create on it.
	Stores []StoreID
}

// Plan is the fully resolved storage layout of one block: which servers to run, which
// logical databases to create, and which instance connects to which.
//
// Producing this as a separate, pure step is deliberate. Store allocation is where the
// subtle mistakes live — a sharer pointed at the wrong owner, two schemas landing in one
// database, a replica silently reusing its sibling's store — and all of them are
// cheaper to catch here than after containers are running.
type Plan struct {
	// Servers is one entry per non-embedded engine used in the block, sorted by type
	// for determinism.
	Servers []Server

	// Stores is every logical database, sorted by ID.
	Stores map[StoreID]*Store

	// Assignments maps each component instance to the store it connects to.
	Assignments map[InstanceKey]StoreID
}

// Store returns a store by ID.
func (p *Plan) Store(id StoreID) (*Store, bool) {
	s, ok := p.Stores[id]
	return s, ok
}

// StoreFor returns the store a component instance connects to.
func (p *Plan) StoreFor(component string, ordinal int) (*Store, bool) {
	id, ok := p.Assignments[KeyFor(component, ordinal)]
	if !ok {
		return nil, false
	}
	return p.Store(id)
}

// StoreIDs returns every store ID, sorted, for deterministic iteration.
func (p *Plan) StoreIDs() []StoreID {
	ids := make([]StoreID, 0, len(p.Stores))
	for id := range p.Stores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	return ids
}

// NeedsServer reports whether the block must run a container for this engine.
func (p *Plan) NeedsServer(t components.DBType) bool {
	for _, s := range p.Servers {
		if s.Type == t {
			return true
		}
	}
	return false
}

// BuildPlan resolves a block's storage requests into a plan.
//
// Allocation rules, each chosen because the alternative fails quietly:
//
//   - One store per component instance by DEFAULT. Two replicas get two independent
//     databases, which is what a multi-gateway test needs and what a hand-written name
//     pair was previously encoding by convention.
//   - A component declaring SharesStoreWith connects to that component's store instead.
//     Sharing is a fixed property of the component, not a per-block choice, so it is
//     not expressible here.
//   - At most one server per non-embedded engine. Embedded engines contribute none.
//   - Physical database names are generated from the component name, so they are
//     deterministic within a block — easier to follow in logs than a random token, and
//     unique because each block has its own server.
func BuildPlan(requests []Request) (*Plan, error) {
	var errs []error
	addf := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	byName := make(map[string]Request, len(requests))
	for _, r := range requests {
		if r.Def == nil {
			addf("runtime: a storage request has no definition")
			continue
		}
		if _, dup := byName[r.Def.Name]; dup {
			addf("runtime: component %q appears twice in one block's storage requests", r.Def.Name)
			continue
		}
		byName[r.Def.Name] = r
	}

	plan := &Plan{
		Stores:      make(map[StoreID]*Store),
		Assignments: make(map[InstanceKey]StoreID),
	}
	databaseOwners := make(map[string]string)

	// Pass 1: create a store for every instance that OWNS its storage. Sharers are
	// resolved afterwards, so a sharer declared before its owner still works.
	names := sortedNames(byName)
	for _, name := range names {
		r := byName[name]
		if r.Def.DB == nil {
			continue
		}
		if !r.Def.DB.Owns() {
			continue
		}
		if err := validateRequest(r); err != nil {
			errs = append(errs, err)
			continue
		}
		for ordinal := range replicasOf(r) {
			store := newStore(r, ordinal)
			if owner, exists := databaseOwners[store.Database]; exists {
				addf("runtime: components %q and %q produce the same database name %q",
					owner, r.Def.Name, store.Database)
			}
			databaseOwners[store.Database] = r.Def.Name
			plan.Stores[store.ID] = store
			plan.Assignments[KeyFor(name, ordinal)] = store.ID
		}
	}

	// Pass 2: point each sharer at its owner's store.
	for _, name := range names {
		r := byName[name]
		if r.Def.DB == nil || r.Def.DB.Owns() {
			continue
		}
		if err := validateRequest(r); err != nil {
			errs = append(errs, err)
			continue
		}
		owner := r.Def.DB.SharesStoreWith
		ownerReq, present := byName[owner]
		if !present {
			// The registry validates that the owner EXISTS; this is the stronger
			// block-level requirement that it is actually present in this topology.
			addf("runtime: %s shares %q's store, but %q is not in this block",
				r.Def, owner, owner)
			continue
		}
		if ownerReq.Type != r.Type {
			// Otherwise the sharer would be handed a DSN for an engine it was not
			// configured for, and the mismatch would surface as a driver error.
			addf("runtime: %s uses %q but shares %q's store, which is %q",
				r.Def, r.Type, owner, ownerReq.Type)
			continue
		}

		ownerReplicas := replicasOf(ownerReq)
		sharerReplicas := replicasOf(r)
		switch {
		case ownerReplicas == 1:
			// Every sharer instance reads the single owner store.
			for ordinal := range sharerReplicas {
				plan.Assignments[KeyFor(name, ordinal)] = KeyStore(owner, 0)
			}
		case ownerReplicas == sharerReplicas:
			// Pair by position: sharer #2 reads owner #2's store.
			for ordinal := range sharerReplicas {
				plan.Assignments[KeyFor(name, ordinal)] = KeyStore(owner, ordinal)
			}
		default:
			// Any other combination has no single obvious pairing, and guessing one
			// would silently connect a test to the wrong data.
			addf("runtime: %s has %d replicas sharing %q's %d stores; "+
				"replica counts must match or the owner must have exactly one",
				r.Def, sharerReplicas, owner, ownerReplicas)
		}
	}

	// Pass 3: one server per non-embedded engine.
	byType := make(map[components.DBType][]StoreID)
	images := make(map[components.DBType]components.ImageRef)
	for _, id := range plan.StoreIDs() {
		s := plan.Stores[id]
		if s.Embedded {
			continue
		}
		byType[s.Type] = append(byType[s.Type], id)
	}
	types := make([]components.DBType, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Slice(types, func(a, b int) bool { return types[a] < types[b] })
	for _, t := range types {
		for _, r := range byName {
			if r.Type != t || r.Image.Ref == "" {
				continue
			}
			if image, exists := images[t]; exists && !reflect.DeepEqual(image, r.Image) {
				addf("runtime: block selects multiple images for engine %q", t)
				continue
			}
			images[t] = r.Image
		}
		plan.Servers = append(plan.Servers, Server{Type: t, Image: images[t], Stores: byType[t]})
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return plan, nil
}

// KeyStore is the store ID owned by a component ordinal.
func KeyStore(component string, ordinal int) StoreID {
	if ordinal == 0 {
		return StoreID(component)
	}
	return StoreID(fmt.Sprintf("%s-%d", component, ordinal+1))
}

func newStore(r Request, ordinal int) *Store {
	schema, _ := r.Def.DB.SchemaFor(r.Type)
	return &Store{
		ID:           KeyStore(r.Def.Name, ordinal),
		Type:         r.Type,
		Database:     databaseName(r.Def.Name, ordinal),
		Owner:        r.Def.Name,
		OwnerOrdinal: ordinal,
		Schema:       schema,
		Embedded:     r.Type.Embedded(),
	}
}

func validateRequest(r Request) error {
	if r.Type == "" {
		return fmt.Errorf("runtime: %s has storage but no engine was resolved for it", r.Def)
	}
	if !r.Type.Valid() {
		return fmt.Errorf("runtime: %s was given unknown engine %q", r.Def, r.Type)
	}
	if !r.Def.DB.Supports(r.Type) {
		return fmt.Errorf("runtime: %s does not support engine %q", r.Def, r.Type)
	}
	return nil
}

func replicasOf(r Request) int {
	if r.Replicas <= 0 {
		return 1
	}
	return r.Replicas
}

func sortedNames(m map[string]Request) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// databaseName converts a component name into a legal, readable database identifier.
//
// Deterministic rather than random: within a block the server is private, so
// block-local uniqueness suffices, and a name that reads like the component it belongs
// to is far easier to follow in a failing test's logs.
func databaseName(component string, ordinal int) string {
	var b strings.Builder
	for _, r := range strings.ToLower(component) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	name := strings.Trim(b.String(), "_")
	if name == "" {
		name = "store"
	}
	if ordinal > 0 {
		name = fmt.Sprintf("%s_%d", name, ordinal+1)
	}
	return name
}

// launchWithRetry starts a database server container, retrying a failed boot up to the
// engine's attempt budget. Every attempt starts a FRESH container — nothing has been
// provisioned yet, so there is no state a retry could mask. Each failure is logged so a
// flake's frequency stays observable; the last error is returned when the budget is spent.
func launchWithRetry(
	ctx context.Context, engineName string, attempts int,
	launch func(context.Context) (*Container, error),
) (*Container, error) {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		container, err := launch(ctx)
		if err == nil {
			return container, nil
		}
		lastErr = err
		if container != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if stopErr := container.Stop(cleanupCtx); stopErr != nil {
				slog.Warn("database server cleanup after failed boot failed",
					"engine", engineName, "attempt", attempt, "error", stopErr)
			}
			cancel()
		}
		if attempt < attempts {
			slog.Warn("database server boot failed; retrying with a fresh container",
				"engine", engineName, "attempt", attempt, "of", attempts, "error", err)
		}
	}
	return nil, lastErr
}

// Provisioned is the outcome of provisioning one block's storage: the server containers
// that were started, and the environment each component instance needs in order to reach
// its store.
type Provisioned struct {
	// Servers are the started database containers, keyed by engine, so the block can
	// stop them at teardown.
	Servers map[components.DBType]*Container

	// Env is the environment to merge into each component instance, keyed by instance.
	// Already rendered through each component's own naming convention.
	Env map[InstanceKey]map[string]string

	// Credentials are the block's generated credentials, exposed for diagnostics and
	// for steps that must connect to a store directly.
	Credentials Credentials

	// Plan is the layout that was provisioned.
	Plan *Plan
}

// DatabaseOptions configures database provisioning.
type DatabaseOptions struct {
	// Network is the block's private network. Required when any server is needed.
	Network *Network

	// RepoRoot resolves the repo-relative schema paths a component registers.
	RepoRoot string

	// Requests is each component's storage need.
	Requests []Request
}

// Provision brings up every database a block needs and returns the environment its
// components require.
//
// The whole of this is invisible to a test author, whose only expression of storage is
// "db: postgres" on a component or a block. Everything here — how many servers, what the
// databases are called, which credentials, which DDL, in what order, by what mechanism —
// is the framework's problem.
//
// On any failure, servers already started are stopped before returning, so a partial
// provision does not leak containers for the rest of the process.
func Provision(ctx context.Context, opts DatabaseOptions) (*Provisioned, error) {
	plan, err := BuildPlan(opts.Requests)
	if err != nil {
		return nil, err
	}

	creds, err := NewCredentials()
	if err != nil {
		return nil, err
	}

	out := &Provisioned{
		Servers:     make(map[components.DBType]*Container),
		Env:         make(map[InstanceKey]map[string]string),
		Credentials: creds,
		Plan:        plan,
	}

	// Start one server per non-embedded engine.
	for _, server := range plan.Servers {
		if opts.Network == nil {
			out.cleanup()
			return nil, fmt.Errorf("runtime: engine %q needs a server, so a network is required", server.Type)
		}

		eng, err := engineFor(server.Type)
		if err != nil {
			out.cleanup()
			return nil, err
		}

		stores := make([]*Store, 0, len(server.Stores))
		for _, id := range server.Stores {
			if s, ok := plan.Store(id); ok {
				stores = append(stores, s)
			}
		}

		def, err := eng.serverDefinition(creds, stores, opts.RepoRoot, server.Image)
		if err != nil {
			out.cleanup()
			return nil, err
		}
		if err := def.Validate(); err != nil {
			// A malformed generated definition is a framework bug; say so rather than
			// letting docker report something obscure.
			out.cleanup()
			return nil, fmt.Errorf("runtime: generated %q server definition is invalid: %w", server.Type, err)
		}

		container, err := launchWithRetry(ctx, string(server.Type), eng.bootAttempts(),
			func(ctx context.Context) (*Container, error) {
				return Launch(ctx, def, Options{
					Network:  opts.Network,
					RepoRoot: opts.RepoRoot,
				})
			})
		if err != nil {
			out.cleanup()
			return nil, fmt.Errorf("runtime: starting the %q server: %w", server.Type, err)
		}
		out.Servers[server.Type] = container

		if !eng.canBootstrapAtStart() {
			// The engine cannot create databases and apply schema at first boot, so
			// PhaseSchemaApplied is reached by connecting afterwards. Dependent
			// components must not start before this returns, or they come up against a
			// database with no tables and report it as a product error.
			if err := eng.applySchema(ctx, container, creds, stores, opts.RepoRoot); err != nil {
				out.cleanup()
				return nil, err
			}
		}
	}

	// Render each instance's environment from its store.
	var errs []error
	for _, r := range opts.Requests {
		if r.Def == nil || r.Def.DB == nil {
			continue
		}
		replicas := replicasOf(r)
		for ordinal := range replicas {
			key := KeyFor(r.Def.Name, ordinal)
			store, ok := plan.StoreFor(r.Def.Name, ordinal)
			if !ok {
				errs = append(errs, fmt.Errorf("runtime: no store was planned for %s", key))
				continue
			}

			dsn, err := out.dsnFor(store, creds)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if r.Def.DB.Env == nil {
				// Only a sharer may omit Env, and only if it truly needs none; the
				// definition validator already rejects an owner without one.
				continue
			}
			out.Env[key] = r.Def.DB.Env(dsn)
		}
	}
	if err := errors.Join(errs...); err != nil {
		out.cleanup()
		return nil, err
	}

	return out, nil
}

// dsnFor builds the coordinates for a store, which differ by engine.
func (p *Provisioned) dsnFor(store *Store, creds Credentials) (components.DSN, error) {
	if p == nil {
		return components.DSN{}, fmt.Errorf("runtime: provisioning is required")
	}
	if store == nil {
		return components.DSN{}, fmt.Errorf("runtime: store is required")
	}
	if store.Embedded {
		// An embedded engine has no host or port. The path is inside the owning
		// component's own container, so it is derived from the store rather than from
		// any server.
		return components.DSN{
			Type:     store.Type,
			Database: store.Database,
			FilePath: fmt.Sprintf("/app/data/%s.db", store.Database),
		}, nil
	}

	eng, err := engineFor(store.Type)
	if err != nil {
		return components.DSN{}, err
	}
	server, ok := p.Servers[store.Type]
	if !ok {
		return components.DSN{}, fmt.Errorf("runtime: no %q server was started for store %q",
			store.Type, store.Database)
	}
	// Components reach the server by network alias and canonical port; the mapped host
	// port is for the test process, never for a container.
	return eng.dsn(store, server.Instance.Alias(), creds), nil
}

// Stop tears down every server this provisioning started.
func (p *Provisioned) Stop(ctx context.Context) error {
	return p.stopAll(ctx)
}

func (p *Provisioned) cleanup() {
	if p == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	_ = p.stopAll(ctx)
}

func (p *Provisioned) stopAll(ctx context.Context) error {
	if p == nil {
		return nil
	}
	var errs []error
	types := make([]components.DBType, 0, len(p.Servers))
	for t := range p.Servers {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	for _, t := range types {
		c := p.Servers[t]
		if err := c.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("database: stopping the %q server: %w", t, err))
			continue
		}
		delete(p.Servers, t)
	}
	return errors.Join(errs...)
}
