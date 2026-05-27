import 'express';

declare module 'express' {
  interface Request {
    requestId?: string;
    user?: any;
    token?: string;
    authType?: string;
  }
}
