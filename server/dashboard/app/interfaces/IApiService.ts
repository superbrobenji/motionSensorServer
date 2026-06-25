export interface IApiResponse<T = any> {
  success: boolean;
  error?: string;
  data?: T;
}

export interface IEnrollment {
  mac: string;
  publicKey: string;
  status: 0 | 1 | 2; // 0=pending, 1=approved, 2=rejected
  receivedAt: number;
  approvedAt: number;
}

export interface ITxPowerStatus {
  preset: number;
  name: string;
}
