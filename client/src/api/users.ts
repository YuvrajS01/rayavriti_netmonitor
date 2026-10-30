import { v1, wrap, type RequestOptions } from './http';

export interface ManagedUser {
  id: number;
  username: string;
  role: string;
  display_name: string;
  email: string;
  phone: string;
  enabled: boolean;
  last_login_at: string | null;
  created_at: string;
  role_id: number;
}

export interface Role {
  id: number;
  name: string;
  display_name: string;
  description: string;
  permissions: string[];
  is_system: boolean;
}

export const getUsers = ({ signal }: RequestOptions = {}) =>
  v1.get('/users', { signal }).then((r) => wrap<ManagedUser[]>(r.data));

export const createUser = (body: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.post('/users', body, { signal }).then((r) => wrap<ManagedUser>(r.data));

export const updateUser = (id: number, body: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.put(`/users/${id}`, body, { signal }).then((r) => wrap<ManagedUser>(r.data));

export const deleteUser = (id: number, { signal }: RequestOptions = {}) =>
  v1.delete(`/users/${id}`, { signal }).then((r) => r.data as unknown);

export const getRoles = ({ signal }: RequestOptions = {}) =>
  v1.get('/roles', { signal }).then((r) => wrap<Role[]>(r.data));

export const createRole = (body: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.post('/roles', body, { signal }).then((r) => wrap<Role>(r.data));

export const updateRole = (id: number, body: Record<string, unknown>, { signal }: RequestOptions = {}) =>
  v1.put(`/roles/${id}`, body, { signal }).then((r) => wrap<Role>(r.data));

export const deleteRole = (id: number, { signal }: RequestOptions = {}) =>
  v1.delete(`/roles/${id}`, { signal }).then((r) => r.data as unknown);
