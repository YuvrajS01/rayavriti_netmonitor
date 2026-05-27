declare module 'node-netflowv9' {
  function Collector(options: { port: number }): any;
  export default Collector;
}

declare module 'ping' {
  const ping: { promise: { probe(host: string, options?: any): Promise<any> } };
  export default ping;
}

declare module 'net-snmp' {
  export const Version1: any;
  export const Version2c: any;
  export function createSession(host: string, community: string, options?: any): any;
  export function isVarbindError(vb: any): boolean;
  export function varbindError(vb: any): string;
  export const PROTOCOL: any;
}
