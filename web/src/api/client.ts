import createClient, { type Middleware } from "openapi-fetch";
import type { components, paths } from "./schema";

export type Schemas = components["schemas"];
export type Entry = Schemas["Entry"];
export type Folder = Schemas["Folder"];
export type User = Schemas["User"];
export type Session = Schemas["Session"];
export type TrashItem = Schemas["TrashItem"];
export type Link = Schemas["Link"];
export type Storage = Schemas["Storage"];
export type Settings = Schemas["Settings"];
export type ErrorCode = Schemas["Error"]["error"]["code"];

export class ApiError extends Error {
  readonly status: number;
  readonly code: ErrorCode;

  constructor(status: number, code: ErrorCode, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }

  static async from(response: Response): Promise<ApiError> {
    try {
      const body = (await response.clone().json()) as Schemas["Error"];
      return new ApiError(response.status, body.error.code, body.error.message);
    } catch {
      return new ApiError(response.status, "internal", response.statusText || "Request failed");
    }
  }
}

type UnauthorizedListener = () => void;
const unauthorizedListeners = new Set<UnauthorizedListener>();

export function onUnauthorized(listener: UnauthorizedListener): () => void {
  unauthorizedListeners.add(listener);
  return () => unauthorizedListeners.delete(listener);
}

const errors: Middleware = {
  async onResponse({ response, request }) {
    if (response.ok) return undefined;
    if (response.status === 401 && !request.url.endsWith("/auth/login")) {
      unauthorizedListeners.forEach((listener) => listener());
    }
    throw await ApiError.from(response);
  },
};

export const api = createClient<paths>({
  baseUrl: `${window.location.origin}/api/v1`,
  credentials: "same-origin",
  fetch: (request) => globalThis.fetch(request),
});
api.use(errors);

export function messageOf(error: unknown): string {
  if (error instanceof ApiError) return error.message.charAt(0).toUpperCase() + error.message.slice(1) + ".";
  if (error instanceof Error) return error.message;
  return "Something went wrong.";
}
