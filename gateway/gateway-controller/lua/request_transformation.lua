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

  -- Check the new SDK format first (mods.Method sets this)
  local method_key = metadata["method"]
  if method_key ~= nil then
    return method_key
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

  -- Check the new SDK format first (mods.Path sets this)
  local path_key = metadata["path"]
  if path_key ~= nil then
    return path_key
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
local function compute_upstream_path(handle, target_path, api_context, upstream_base_path)
  if target_path == nil or target_path == "" then
    return nil
  end

  -- Default values
  api_context = api_context or ""
  upstream_base_path = upstream_base_path or ""

  handle:logInfo("compute_upstream_path: target_path=" .. tostring(target_path) .. 
    " api_context=" .. tostring(api_context) .. 
    " upstream_base_path=" .. tostring(upstream_base_path))

  -- Strip the context prefix from target_path
  local relative_path = target_path
  if api_context ~= "" and api_context ~= "/" then
    -- Check if target_path starts with api_context
    local prefix = string.sub(target_path, 1, #api_context)
    handle:logInfo("compute_upstream_path: checking prefix=" .. tostring(prefix))
    if prefix == api_context then
      relative_path = string.sub(target_path, #api_context + 1)
      if relative_path == "" then
        relative_path = "/"
      end
      handle:logInfo("compute_upstream_path: stripped to relative_path=" .. tostring(relative_path))
    else
      handle:logInfo("compute_upstream_path: prefix mismatch, not stripping")
    end
  else
    handle:logInfo("compute_upstream_path: api_context empty or /, not stripping")
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
    -- Note: handle:metadata() only works for envoy.filters.http.lua namespace
    -- For custom namespaces like wso2.route, we need to use filterMetadata from route
    local api_context = nil
    local upstream_base_path = nil

    -- Access route metadata via streamInfo's filterState or by reading from dynamic metadata
    -- Since Lua filter's handle:metadata() only returns envoy.filters.http.lua namespace,
    -- we store api_context in dynamic metadata from ext_proc
    if extproc_metadata ~= nil then
      local ctx_val = extproc_metadata["api_context"]
      if ctx_val ~= nil then
        api_context = tostring(ctx_val)
      end
      local up_val = extproc_metadata["upstream_base_path"]
      if up_val ~= nil then
        upstream_base_path = tostring(up_val)
      end
      -- If a target upstream was set by dynamic-endpoint policy, use its base path instead
      local target_base = extproc_metadata["target_upstream_base_path"]
      if target_base ~= nil then
        local target_base_str = tostring(target_base)
        if target_base_str ~= "" and target_base_str ~= "nil" then
          upstream_base_path = target_base_str
        end
      end
    end

    handle:logInfo("path_rewrite: target_path=" .. tostring(target_path) .. 
      " api_context=" .. tostring(api_context) .. 
      " upstream_base_path=" .. tostring(upstream_base_path))

    -- Compute the final upstream path
    local final_path = compute_upstream_path(handle, target_path, api_context, upstream_base_path)
    if final_path ~= nil then
      handle:logInfo("path_rewrite: final_path=" .. tostring(final_path))
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
