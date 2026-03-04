-- Request transformation Lua filter for HTTP method and path rewriting.
-- Reads target values from dynamic metadata and replaces the corresponding headers.
-- This filter runs AFTER routing, so path rewrites here do NOT trigger route re-evaluation.

local allowed_methods = {
  GET = true,
  POST = true,
  PUT = true,
  DELETE = true,
  PATCH = true,
  HEAD = true,
  OPTIONS = true,
}

local function normalize_method(value)
  if type(value) ~= "string" then
    return nil
  end
  return string.upper(value)
end

local function resolve_target_method(metadata)
  if metadata == nil then
    return nil
  end

  local direct = metadata["request_transformation.target_method"]
  if direct ~= nil then
    return direct
  end

  local new_method = metadata["new_method"]
  if new_method ~= nil then
    return new_method
  end

  local nested = metadata["request_transformation"]
  if type(nested) == "table" then
    return nested["target_method"]
  end

  return nil
end

local function resolve_target_path(metadata)
  if metadata == nil then
    return nil
  end

  local direct = metadata["request_transformation.target_path"]
  if direct ~= nil then
    return direct
  end

  local nested = metadata["request_transformation"]
  if type(nested) == "table" then
    return nested["target_path"]
  end

  return nil
end

function envoy_on_request(handle)
  local stream_info = handle:streamInfo()
  if stream_info == nil then
    return
  end

  local dynamic_metadata = stream_info:dynamicMetadata()
  local extproc_metadata = nil
  if dynamic_metadata ~= nil then
    extproc_metadata = dynamic_metadata:get("api_platform.policy_engine.envoy.filters.http.ext_proc")
  end

  -- Handle path rewriting (includes query string if present)
  local target_path = resolve_target_path(extproc_metadata)
  if target_path ~= nil and type(target_path) == "string" and target_path ~= "" then
    handle:headers():replace(":path", target_path)
  end

  -- Handle HTTP method rewriting
  local target_method = resolve_target_method(extproc_metadata)
  if target_method == nil then
    return
  end

  local normalized = normalize_method(target_method)
  if normalized == nil then
    handle:logWarn("request transformation: target_method is not a string")
    return
  end

  if not allowed_methods[normalized] then
    handle:logWarn("request transformation: invalid target_method: " .. tostring(target_method))
    return
  end

  handle:headers():replace(":method", normalized)
end
