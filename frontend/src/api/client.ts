// 统一响应封装 {code, message, data}（与 Go httpx/respond.go 对齐）
export interface ApiResponse<T> {
  code: number | string
  message: string
  data: T
}

export class ApiError extends Error {
  constructor(
    public code: number | string,
    message: string,
  ) {
    super(message)
  }
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response
  try {
    res = await fetch(`/api/v1${path}`, {
      headers: { 'Content-Type': 'application/json' },
      ...init,
    })
  } catch {
    throw new ApiError('network', '网络请求失败，请检查后端服务是否运行')
  }
  let body: ApiResponse<T>
  try {
    body = (await res.json()) as ApiResponse<T>
  } catch {
    throw new ApiError(res.status, `HTTP ${res.status}（响应不是合法 JSON）`)
  }
  if (body.code !== 0) {
    throw new ApiError(body.code, body.message || `HTTP ${res.status}`)
  }
  return body.data
}

export const get = <T,>(path: string) => api<T>(path)
export const post = <T,>(path: string, data?: unknown) =>
  api<T>(path, { method: 'POST', body: data !== undefined ? JSON.stringify(data) : undefined })
export const put = <T,>(path: string, data?: unknown) =>
  api<T>(path, { method: 'PUT', body: data !== undefined ? JSON.stringify(data) : undefined })
export const del = <T,>(path: string) => api<T>(path, { method: 'DELETE' })
