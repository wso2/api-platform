// ---------------------------------------------------------
// Shared Error Parsing Utility
// ---------------------------------------------------------

export const parseApiError = async (
  response: Response,
  operation: string
): Promise<string> => {
  let errorMessage = `Failed to ${operation} (${response.status})`;

  const text = await response.text();
  try {
    const errorData = JSON.parse(text);
    if (errorData.description) {
      errorMessage = errorData.description;
    } else if (errorData.message) {
      errorMessage = errorData.message;
    } else if (errorData.error) {
      errorMessage = errorData.error;
    } else if (errorData.detail) {
      errorMessage = errorData.detail;
    } else if (text.trim()) {
      // Fallback to raw text if JSON parsed but no useful fields
      errorMessage = text.trim();
    }
  } catch {
    // JSON parsing failed - use raw text if available
    if (text.trim()) {
      errorMessage = text.trim();
    }
  }

  return errorMessage;
};
