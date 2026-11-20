/** ApiError is a failed call to the API: code is for the front to react to, message is shown to the user. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

/** UNAVAILABLE is the code of an error without an answer of the API: network down, server stopped. */
export const UNAVAILABLE = "unavailable";

const unavailableMessage = "Le serveur ne répond pas, réessayez plus tard.";

/** request calls the API at path, sending body as JSON, and returns the decoded response. */
export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api${path}`, {
      method,
      headers: body === undefined ? undefined : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, UNAVAILABLE, unavailableMessage);
  }
  if (!response.ok) {
    throw await toApiError(response);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

/** toApiError reads the error of a failed response, or makes one when the API didn't answer it. */
export async function toApiError(response: Response): Promise<ApiError> {
  try {
    const body: unknown = await response.json();
    if (isErrorBody(body)) {
      return new ApiError(response.status, body.error, body.message);
    }
  } catch {
    // Not JSON: the error page of a reverse proxy
  }
  return new ApiError(response.status, UNAVAILABLE, unavailableMessage);
}

function isErrorBody(body: unknown): body is { error: string; message: string } {
  return (
    typeof body === "object" &&
    body !== null &&
    "error" in body &&
    typeof body.error === "string" &&
    "message" in body &&
    typeof body.message === "string"
  );
}
