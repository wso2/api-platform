// ---------------------------------------------------------
// Shared Error Parsing Utility
// ---------------------------------------------------------

export const parseApiError = async (
  response: Response,
  operation: string
): Promise<string> => {
  let errorMessage = `Failed to ${operation} (${response.status})`;

  try {
    const errorData = await response.json();
    if (errorData.description) {
      errorMessage = errorData.description;
    } else if (errorData.message) {
      errorMessage = errorData.message;
    } else if (errorData.error) {
      errorMessage = errorData.error;
    } else if (errorData.detail) {
      errorMessage = errorData.detail;
    }
  } catch {
    try {
      const errorText = await response.text();
      if (errorText) {
        errorMessage = errorText;
      }
    } catch {
      // Keep default message
    }
  }

  return errorMessage;
};
