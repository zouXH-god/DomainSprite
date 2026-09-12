export interface Account {
  name: string;
  type: string;
  accessKeyId: string;
}
export interface Domain {
  id: string;
  domainName: string;
  dnsFrom: string;
  accountName: string;
  status: string;
  certificateId: number;
}
export interface RecordInfo {
  id: string;
  domainId: string;
  domainName: string;
  recordName: string;
  recordType: string;
  recordContent: string;
  line: string;
  status: string;
  ttl: number;
  proxied: boolean;
  updateTime: string;
}
export interface Certificate {
  id: number;
  state: string;
  stage: string;
  taskId: string;
  commonName: string;
  domainList: string;
  notAfter: string;
  lineageId: string;
  parentId: number;
  lastError: string;
}
export interface Task {
  id: number;
  certId: number;
  taskId: string;
  state: string;
  kind: string;
  attempt: number;
  result: string;
  createTime: string;
}
export interface Page<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
}
