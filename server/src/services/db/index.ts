/**
 * Database abstraction layer.
 *
 * Currently re-exports the SQLite adapter. To switch to Postgres+TimescaleDB,
 * replace this re-export with the postgres adapter.
 */
export type { IDatabase } from './types';
export { default } from './sqlite';
