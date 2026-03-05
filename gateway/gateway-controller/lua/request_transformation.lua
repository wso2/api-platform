-- Request transformation Lua filter for HTTP method and path rewriting.
-- Reads target values from dynamic metadata and replaces the corresponding headers.
-- This filter runs AFTER routing, so path rewrites here do NOT trigger route re-evaluation.
-- For path rewrites, Lua strips the API context and prepends the upstream base path since
-- Envoy's route-level path rewrite is not re-applied when we modify :path with clear_route_cache=false.

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

-- Strip API context from path and prepend upstream base path
-- E.g., context="/weather/v1.0", upstream="/anything", path="/weather/v1.0/api/v2"
-- returns "/anything/api/v2"
local function compute_upstream_path(target_path, api_context, upstream_base_path)
  if target_path == nil or target_path == "" then
    return nil
  end

  -- Default values
  api_context = api_context or ""
  upstream_base_path = upstream_base_path or ""

  -- Strip the context prefix from target_path
  local relative_path = target_path
  if api_context ~= "" and api_context ~= "/" then
    -- Check if target_path starts with api_context
    if string.sub(target_path, 1, #api_context) == api_context then
      relative_path = string.sub(target_path, #api_context + 1)
      if relative_path == "" then
        relative_path = "/"
      end
    end
  end

  -- Prepend upstream base path
  if upstream_base_path == "" or upstream_base_path == "/" then
    return relative_path
  end

  -- Handle trailing slash in upstream_base_path and leading slash in relative_path
  if string.sub(upstream_base_path, -1) == "/" and string.sub(relative_path, 1, 1) == "/" then
    return upstream_base_path .. string.sub(relative_path, 2)
  elseif string.sub(upstream_base_path, -1) ~= "/" and string.sub(relative_path, 1, 1) ~= "/" then
    return upstream_base_path .. "/" .. relative_path
  else
    return upstream_base_path .. relative_path
  end
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

  -- Handle path rewriting
  local target_path = resolve_target_path(extproc_metadata)
  if target_path ~= nil and type(target_path) == "string" and target_path ~= "" then
    -- Get route metadata for context stripping and upstream path prepending
    local route_metadata = handle:metadata()
    local api_context = nil
    local upstream_base_path = nil

    if route_metadata ~= nil then
      local wso2_route = route_metadata:get("wso2.route")
      if wso2_route ~= nil then
        api_context = wso2_route["api_context"]
        upstream_base_path = wso2_route["upstream_base_path"]
      end
    end

    -- Compute the final upstream path
    local final_path = compute_upstream_path(target_path, api_context, upstream_base_path)
    if final_path ~= nil then
      handle:headers():replace(":path", final_path)
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
