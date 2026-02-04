-- Request transformation Lua filter for HTTP method rewriting.
-- Reads target method from dynamic metadata and replaces the :method header.

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

function envoy_on_request(handle)
  local stream_info = handle:streamInfo()
  if stream_info == nil then
    return
  end

  local dynamic_metadata = stream_info:dynamicMetadata()
  if dynamic_metadata == nil then
    return
  end

  local extproc_metadata = dynamic_metadata:get("api_platform.policy_engine.envoy.filters.http.ext_proc")
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
