# CountLetters Policy

The **CountLetters** policy analyzes the response body from the upstream service and counts occurrences of specified letters or character patterns. It replaces the original response body with the count results in either JSON or plain text format.

## Policy Type

- **Phase**: Response only
- **Execution**: Continues to next policy (does not short-circuit)
- **Body Processing**: Requires response body

## Use Cases

- Content analysis and statistics
- Debugging and testing response data
- Character frequency analysis
- Pattern occurrence counting
- Monitoring specific keywords or patterns in responses
- Data validation and quality checks
- Educational tools for text analysis

## Configuration Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `letters` | array of strings | Yes | - | Letters/characters/patterns to count |
| `caseSensitive` | boolean | No | `false` | Whether counting is case-sensitive |
| `outputFormat` | string | No | `"json"` | Output format: `"json"` or `"text"` |

## Examples

### Count Vowels (Case-Insensitive)

Count all vowels in the response body:

```yaml
- name: CountLetters
  version: v1.0.0
  enabled: true
  parameters:
    letters: ["a", "e", "i", "o", "u"]
    caseSensitive: false
    outputFormat: json
```

**Original Response:**
```
Hello World! This is a sample text for analysis.
```

**Modified Response:**
```json
{
  "letterCounts": {
    "a": 5,
    "e": 3,
    "i": 3,
    "o": 2,
    "u": 0
  }
}
```

### Count Specific Characters (Case-Sensitive)

Count uppercase and lowercase letters separately:

```yaml
- name: CountLetters
  version: v1.0.0
  enabled: true
  parameters:
    letters: ["A", "a", "E", "e", "H", "h"]
    caseSensitive: true
    outputFormat: json
```

**Original Response:**
```
Hello World! Happy Days Are Here.
```

**Modified Response:**
```json
{
  "letterCounts": {
    "A": 1,
    "a": 3,
    "E": 0,
    "e": 3,
    "H": 2,
    "h": 0
  }
}
```

### Count Words or Patterns

Count multi-character strings (useful for keyword analysis):

```yaml
- name: CountLetters
  version: v1.0.0
  enabled: true
  parameters:
    letters: ["error", "success", "warning", "info"]
    caseSensitive: false
    outputFormat: json
```

**Original Response:**
```json
{
  "status": "success",
  "message": "Operation completed successfully",
  "errors": []
}
```

**Modified Response:**
```json
{
  "letterCounts": {
    "error": 1,
    "success": 2,
    "warning": 0,
    "info": 0
  }
}
```

### Count Special Characters

Analyze special character usage:

```yaml
- name: CountLetters
  version: v1.0.0
  enabled: true
  parameters:
    letters: ["@", "#", "$", "!", "?"]
    caseSensitive: true
    outputFormat: json
```

### Text Output Format

Return results as plain text instead of JSON:

```yaml
- name: CountLetters
  version: v1.0.0
  enabled: true
  parameters:
    letters: ["a", "e", "i", "o", "u"]
    caseSensitive: false
    outputFormat: text
```

**Modified Response:**
```
Letter Counts:
a: 5
e: 3
i: 3
o: 2
u: 0
```

### Count Numbers

Count digit occurrences:

```yaml
- name: CountLetters
  version: v1.0.0
  enabled: true
  parameters:
    letters: ["0", "1", "2", "3", "4", "5", "6", "7", "8", "9"]
    caseSensitive: true
    outputFormat: json
```

### Monitor API Response Keywords

Check for error-related keywords in responses:

```yaml
- name: CountLetters
  version: v1.0.0
  enabled: true
  parameters:
    letters: ["exception", "null", "undefined", "failed"]
    caseSensitive: false
    outputFormat: json
```

## Behavior

1. **No Response Body**: If the response body is empty or not present, returns an empty count result
2. **Case Sensitivity**:
   - When `caseSensitive: false` (default), "A" and "a" are counted together
   - When `caseSensitive: true`, "A" and "a" are counted separately
3. **Multi-character Patterns**: The `letters` array can contain multi-character strings for pattern counting
4. **Output Format**:
   - `json`: Returns structured JSON with `letterCounts` object
   - `text`: Returns human-readable plain text with line-separated counts
5. **Original Body Replacement**: The original response body is completely replaced with the count results
6. **Content-Type Header**: Not automatically modified - may need ModifyHeaders policy to set appropriate content-type

## Integration Example

Combine with other policies for analysis pipeline:

```yaml
- route_key: /api/v1/analytics
  policies:
    - name: APIKeyValidation
      version: v1.0.0
      enabled: true
      parameters:
        headerName: x-api-key

    - name: CountLetters
      version: v1.0.0
      enabled: true
      parameters:
        letters: ["error", "success", "warning"]
        caseSensitive: false
        outputFormat: json

    - name: ModifyHeaders
      version: v1.0.0
      enabled: true
      parameters:
        responseHeaders:
          - action: SET
            name: content-type
            value: application/json
          - action: SET
            name: x-analysis-type
            value: letter-count
```

In this example:
1. API key is validated
2. Response body is analyzed for specific keywords
3. Response headers are set to indicate JSON content type
4. Original response is replaced with analysis results

## Performance Considerations

- The policy reads the entire response body into memory
- Counting is performed using Go's `strings.Count()` function (efficient)
- Large response bodies will increase memory usage
- JSON marshaling adds minimal overhead
- For high-throughput APIs, consider impact of body replacement

## Use Cases by Industry

### E-commerce
Count product-related keywords in API responses to analyze catalog data.

### Monitoring & Debugging
Track error keywords in responses for alerting and debugging.

### Content Moderation
Count occurrences of flagged words or patterns in user-generated content.

### Data Quality
Validate response data by counting expected vs. unexpected character patterns.

### Education
Demonstrate text analysis and character frequency algorithms.

## Important Notes

- **Body Replacement**: This policy COMPLETELY replaces the original response body with count results
- **Content Type**: The policy does not modify the `Content-Type` header - use ModifyHeaders policy if needed
- **Large Bodies**: Be cautious with very large response bodies as they are loaded into memory
- **Pattern Overlap**: If counting patterns like "aa" in "aaa", overlapping matches are counted (standard string counting behavior)
- **Empty Results**: If no letters are found, counts will be 0 for each specified letter

## Debugging Tips

1. Use `outputFormat: text` for easier human-readable debugging
2. Start with simple single-character counts before complex patterns
3. Use case-sensitive mode to distinguish between uppercase and lowercase
4. Test with small response bodies first to verify behavior
