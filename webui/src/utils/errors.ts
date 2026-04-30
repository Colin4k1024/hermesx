export interface ApiErrorShape {
  message: string
  status: number
}

export function normalizeApiError(e: unknown): ApiErrorShape {
  if (e instanceof ApiError) return e
  if (e instanceof Error) return { message: e.message, status: 0 }
  return { message: String(e), status: 0 }
}

export class ApiError extends Error {
  status: number
  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}
