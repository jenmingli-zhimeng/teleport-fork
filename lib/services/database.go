/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package services

import (
	"context"
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// DatabaseGetter defines interface for fetching database resources.
type DatabaseGetter interface {
	// GetDatabases returns all database resources.
	GetDatabases(context.Context) ([]types.Database, error)
	// ListDatabases returns a page of database resources.
	ListDatabases(ctx context.Context, limit int, startKey string) ([]types.Database, string, error)
	// RangeDatabases returns database resources within the range [start, end).
	RangeDatabases(ctx context.Context, start, end string) iter.Seq2[types.Database, error]
	// GetDatabase returns the specified database resource.
	GetDatabase(ctx context.Context, name string) (types.Database, error)
}

// Databases defines an interface for managing database resources.
type Databases interface {
	// DatabaseGetter provides methods for fetching database resources.
	DatabaseGetter
	// CreateDatabase creates a new database resource.
	CreateDatabase(context.Context, types.Database) error
	// UpdateDatabase updates an existing database resource.
	UpdateDatabase(context.Context, types.Database) error
	// DeleteDatabase removes the specified database resource.
	DeleteDatabase(ctx context.Context, name string) error
	// DeleteAllDatabases removes all database resources.
	DeleteAllDatabases(context.Context) error
}

// MarshalDatabase marshals the database resource to JSON.
func MarshalDatabase(database types.Database, opts ...MarshalOption) ([]byte, error) {
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch database := database.(type) {
	case *types.DatabaseV3:
		if err := database.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}

		return utils.FastMarshal(maybeResetProtoRevision(cfg.PreserveRevision, database))
	default:
		return nil, trace.BadParameter("unsupported database resource %T", database)
	}
}

// UnmarshalDatabase unmarshals the database resource from JSON.
func UnmarshalDatabase(data []byte, opts ...MarshalOption) (types.Database, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing database resource data")
	}
	cfg, err := CollectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h types.ResourceHeader
	if err := utils.FastUnmarshal(data, &h); err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case types.V3:
		var database types.DatabaseV3
		if err := utils.FastUnmarshal(data, &database); err != nil {
			return nil, trace.BadParameter("%s", err)
		}
		if err := database.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		if cfg.Revision != "" {
			database.SetRevision(cfg.Revision)
		}
		if !cfg.Expires.IsZero() {
			database.SetExpiry(cfg.Expires)
		}
		return &database, nil
	}
	return nil, trace.BadParameter("unsupported database resource version %q", h.Version)
}

const (
	// RDSDescribeTypeInstance is used to filter for Instances type of RDS DBs when describing RDS Databases.
	RDSDescribeTypeInstance = "instance"
	// RDSDescribeTypeCluster is used to filter for Clusters type of RDS DBs when describing RDS Databases.
	RDSDescribeTypeCluster = "cluster"
)

const (
	// RDSEngineMySQL is RDS engine name for MySQL instances.
	RDSEngineMySQL = "mysql"
	// RDSEnginePostgres is RDS engine name for Postgres instances.
	RDSEnginePostgres = "postgres"
	// RDSEngineMariaDB is RDS engine name for MariaDB instances.
	RDSEngineMariaDB = "mariadb"
	// RDSEngineAurora is RDS engine name for Aurora MySQL 5.6 compatible clusters.
	// This reached EOF on Feb 28, 2023.
	// https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.MySQL56.EOL.html
	RDSEngineAurora = "aurora"
	// RDSEngineAuroraMySQL is RDS engine name for Aurora MySQL 5.7 compatible clusters.
	RDSEngineAuroraMySQL = "aurora-mysql"
	// RDSEngineAuroraPostgres is RDS engine name for Aurora Postgres clusters.
	RDSEngineAuroraPostgres = "aurora-postgresql"
)

const (
	// RDSEngineModeProvisioned is the RDS engine mode for provisioned Aurora clusters
	RDSEngineModeProvisioned = "provisioned"
	// RDSEngineModeServerless is the RDS engine mode for Aurora Serverless DB clusters
	RDSEngineModeServerless = "serverless"
	// RDSEngineModeParallelQuery is the RDS engine mode for Aurora MySQL clusters with parallel query enabled
	RDSEngineModeParallelQuery = "parallelquery"
)

const (
	// RDSProxyMySQLPort is the port that RDS Proxy listens on for MySQL connections.
	RDSProxyMySQLPort = 3306
	// RDSProxyPostgresPort is the port that RDS Proxy listens on for Postgres connections.
	RDSProxyPostgresPort = 5432
	// RDSProxySQLServerPort is the port that RDS Proxy listens on for SQL Server connections.
	RDSProxySQLServerPort = 1433
)

const (
	// AzureEngineMySQL is the Azure engine name for MySQL single-server instances.
	AzureEngineMySQL = "Microsoft.DBforMySQL/servers"
	// AzureEngineMySQLFlex is the Azure engine name for MySQL flexible-server instances.
	AzureEngineMySQLFlex = "Microsoft.DBforMySQL/flexibleServers"
	// AzureEnginePostgres is the Azure engine name for PostgreSQL single-server instances.
	AzureEnginePostgres = "Microsoft.DBforPostgreSQL/servers"
	// AzureEnginePostgresFlex is the Azure engine name for PostgreSQL flexible-server instances.
	AzureEnginePostgresFlex = "Microsoft.DBforPostgreSQL/flexibleServers"
)

const (
	// RedshiftServerlessWorkgroupEndpoint is the endpoint type for workgroups.
	RedshiftServerlessWorkgroupEndpoint = "workgroup"
	// RedshiftServerlessVPCEndpoint is the endpoint type for VCP endpoints.
	RedshiftServerlessVPCEndpoint = "vpc-endpoint"
)
