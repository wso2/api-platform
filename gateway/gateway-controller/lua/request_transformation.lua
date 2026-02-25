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

local function resolve_target_upstream_cluster(metadata)
  if metadata == nil then
    return nil
  end

  -- Look for target_upstream_cluster in the metadata
  local cluster = metadata["target_upstream_cluster"]
  if cluster ~= nil and type(cluster) == "string" then
    return cluster
  end

  return nil
end

-- Get default upstream cluster from route metadata
local function get_default_upstream_cluster(handle)
  local route_metadata = handle:metadata()
  if route_metadata == nil then
    return nil
  end

  local wso2_route = route_metadata:get("wso2.route")
  if wso2_route == nil then
    return nil
  end

  local default_cluster = wso2_route["default_upstream_cluster"]
  if default_cluster ~= nil and type(default_cluster) == "string" then
    return default_cluster
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

  -- Handle dynamic upstream cluster routing
  -- Security: Always set x-target-upstream to a controlled value when cluster_header routing is enabled.
  -- This prevents client-supplied headers from influencing cluster selection if ext_proc fails.
  local default_cluster = get_default_upstream_cluster(handle)
  if default_cluster ~= nil then
    -- Route uses cluster_header routing - always set header to controlled value
    -- First, check if ext_proc provided a target cluster
    local target_cluster = resolve_target_upstream_cluster(extproc_metadata)
    if target_cluster ~= nil then
      handle:headers():replace("x-target-upstream", target_cluster)
    else
      -- Fallback to default cluster (also handles ext_proc failure case)
      handle:headers():replace("x-target-upstream", default_cluster)
    end
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
