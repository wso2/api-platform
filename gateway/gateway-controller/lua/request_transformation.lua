-- Request transformation Lua filter for method and upstream rewrites.
-- Reads metadata from ext_proc namespace and applies per-request mutations.

local metadata_namespace = "api_platform.policy_engine.envoy.filters.http.ext_proc"
local dynamic_upstream_scheme_header = "x-ap-platform-dynamic-upstream-scheme"

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

local function to_integer(value)
  if value == nil then
    return nil
  end

  if type(value) == "number" then
    if value < 1 or value > 65535 then
      return nil
    end
    local floored = math.floor(value)
    if floored ~= value then
      return nil
    end
    return floored
  end

  if type(value) == "string" then
    local parsed = tonumber(value)
    if parsed == nil then
      return nil
    end
    local floored = math.floor(parsed)
    if floored ~= parsed then
      return nil
    end
    if floored < 1 or floored > 65535 then
      return nil
    end
    return floored
  end

  return nil
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

  local policy_metadata = metadata["policy_metadata"]
  if type(policy_metadata) == "table" then
    local policy_direct = policy_metadata["request_transformation.target_method"]
    if policy_direct ~= nil then
      return policy_direct
    end

    local policy_nested = policy_metadata["request_transformation"]
    if type(policy_nested) == "table" then
      return policy_nested["target_method"]
    end
  end

  return nil
end

local function resolve_upstream_config(metadata)
  if type(metadata) ~= "table" then
    return nil, "dynamic metadata is not an object"
  end

  local request_transformation = metadata["request_transformation"]
  if type(request_transformation) ~= "table" then
    local policy_metadata = metadata["policy_metadata"]
    if type(policy_metadata) == "table" then
      request_transformation = policy_metadata["request_transformation"]
    end
  end

  if type(request_transformation) ~= "table" then
    return nil, "request_transformation metadata not found"
  end

  local upstream = request_transformation["upstream"]
  if type(upstream) ~= "table" then
    return nil, "request_transformation.upstream metadata not found"
  end

  local host = upstream["host"]
  if type(host) ~= "string" or host == "" then
    return nil, "request_transformation.upstream.host is required"
  end

  local scheme = upstream["scheme"]
  if scheme == nil then
    scheme = "http"
  end
  if type(scheme) ~= "string" then
    return nil, "request_transformation.upstream.scheme must be a string"
  end
  scheme = string.lower(scheme)
  if scheme ~= "http" and scheme ~= "https" then
    return nil, "request_transformation.upstream.scheme must be http or https"
  end

  local port = to_integer(upstream["port"])
  if port == nil then
    if upstream["port"] ~= nil then
      return nil, "request_transformation.upstream.port must be an integer between 1 and 65535"
    end
    if scheme == "https" then
      port = 443
    else
      port = 80
    end
  end

  local authority = upstream["authority"]
  if type(authority) ~= "string" or authority == "" then
    authority = host .. ":" .. tostring(port)
  end

  return {
    host = host,
    port = port,
    scheme = scheme,
    authority = authority,
  }, nil
end

function envoy_on_request(handle)
  local stream_info = handle:streamInfo()
  if stream_info == nil then
    return
  end

  local headers = handle:headers()
  local route_cache_dirty = false

  -- Do not trust client-provided internal routing header.
  if headers:get(dynamic_upstream_scheme_header) ~= nil then
    headers:remove(dynamic_upstream_scheme_header)
    route_cache_dirty = true
  end

  local dynamic_metadata = stream_info:dynamicMetadata()
  if dynamic_metadata == nil then
    if route_cache_dirty then
      handle:clearRouteCache()
    end
    return
  end

  local extproc_metadata = dynamic_metadata:get(metadata_namespace)

  local target_method = resolve_target_method(extproc_metadata)
  if target_method ~= nil then
    local normalized = normalize_method(target_method)
    if normalized == nil then
      handle:logWarn("request transformation: target_method is not a string")
    elseif not allowed_methods[normalized] then
      handle:logWarn("request transformation: invalid target_method: " .. tostring(target_method))
    else
      handle:headers():replace(":method", normalized)
    end
  end

  local upstream, upstream_err = resolve_upstream_config(extproc_metadata)
  if upstream == nil then
    if upstream_err ~= "request_transformation metadata not found" and upstream_err ~= "request_transformation.upstream metadata not found" then
      handle:logWarn("request transformation: upstream metadata ignored: " .. upstream_err)
    end
    if route_cache_dirty then
      handle:clearRouteCache()
    end
    return
  end

  headers:replace(":authority", upstream.authority)
  headers:replace(dynamic_upstream_scheme_header, upstream.scheme)
  route_cache_dirty = true

  if route_cache_dirty then
    -- Header-based route selection needs a route cache refresh after header mutation.
    handle:clearRouteCache()
  end
end
