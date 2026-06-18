-- Cache to store container ID -> name mappings
local container_cache = {}

function read_file(path)
    local file = io.open(path, "r")
    if not file then
        return nil, "cannot open file"
    end
    local content = file:read("*all")
    file:close()
    return content, nil
end

function extract_container_name(json_content)
    -- Simple JSON parser to extract "Name" field
    -- Looking for: "Name":"/container-name" or "Name":"container-name"
    local name = json_content:match('"Name"%s*:%s*"/?([^"]+)"')
    return name
end

function add_component_name(tag, timestamp, record)
    local component = nil

    -- Try to extract from container log path
    local log_path = record["_container_log_path"]

    if log_path then
        -- Extract container ID from path
        -- Path format: /var/lib/docker/containers/<container_id>/<container_id>-json.log
        local container_id = log_path:match("/containers/(%x+)/")

        if container_id then
            -- Check cache first
            if container_cache[container_id] then
                component = container_cache[container_id]
            else
                -- Read container config file
                local config_path = "/var/lib/docker/containers/" .. container_id .. "/config.v2.json"
                local config_content, err = read_file(config_path)

                if config_content then
                    -- Extract container name from JSON
                    component = extract_container_name(config_content)

                    -- Cache the result
                    if component and component ~= "" then
                        container_cache[container_id] = component
                    end
                end
            end
        end

        -- Remove the path field from output
        record["_container_log_path"] = nil
    end

    -- Fallback: try to extract from hostname if present
    if not component or component == "" then
        if record["hostname"] then
            component = record["hostname"]
            -- Remove hostname from output since we're using it as component
            record["hostname"] = nil
        end
    end

    -- Set component field
    if component and component ~= "" then
        record["component"] = component
    else
        record["component"] = "unknown"
    end

    return 2, timestamp, record
end
