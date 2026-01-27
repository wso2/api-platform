Feature: LLM Provider Metadata XDS
    As an API developer
    I want LLM provider metadata to be synchronized with the Policy Engine
    So that policies can correctly resolve templates based on provider names

    Background:
        Given the Gateway is running

    Scenario: LLM Provider to Template mapping synchronization
        When I create this LLM provider:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: LlmProvider
            metadata:
                name: testing-openai-provider
            spec:
                displayName: Testing OpenAI Provider
                version: v1.0
                template: openai
                vhost: api.openai.local
                upstream:
                    url: https://api.openai.com/v1
            """
        Then the Policy Engine metadata dump should contain "testing-openai-provider" mapping to "openai"

    Scenario: LLM Provider update synchronization
        Given I create this LLM provider:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: LlmProvider
            metadata:
                name: update-test-provider
            spec:
                displayName: Update Test Provider
                version: v1.0
                template: openai
                vhost: api.openai.local
                upstream:
                    url: https://api.openai.com/v1
            """
        When I update the LLM provider "update-test-provider" with:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: LlmProvider
            metadata:
                name: update-test-provider
            spec:
                displayName: Update Test Provider
                version: v1.0
                template: azure-openai
                vhost: api.openai.local
                upstream:
                    url: https://api.openai.com/v1
            """
        Then the Policy Engine metadata dump should contain "update-test-provider" mapping to "azure-openai"

    Scenario: LLM Provider deletion synchronization
        Given I create this LLM provider:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: LlmProvider
            metadata:
                name: delete-test-provider
            spec:
                displayName: Delete Test Provider
                version: v1.0
                template: openai
                vhost: api.openai.local
                upstream:
                    url: https://api.openai.com/v1
            """
        When I delete the LLM provider "delete-test-provider"
        Then the Policy Engine metadata dump should not contain provider "delete-test-provider"
