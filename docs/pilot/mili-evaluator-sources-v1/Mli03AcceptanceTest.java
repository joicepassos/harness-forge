package com.mili.core.webhook.pilot;

import com.mili.core.tenant.TenantId;
import com.mili.core.tenant.TenantService;
import com.mili.core.webhook.WebhookInboundService;
import com.mili.core.webhook.api.WebhookSummary;
import com.mili.core.webhook.internal.domain.AuthResult;
import com.mili.core.webhook.internal.domain.IngestToken;
import com.mili.core.webhook.internal.domain.IpAllowlist;
import com.mili.core.webhook.internal.domain.WebhookId;
import com.mili.core.webhook.internal.infrastructure.cache.CachedWebhookConfig;
import com.mili.core.webhook.internal.infrastructure.cache.WebhookInboundCache;
import com.mili.core.webhook.internal.infrastructure.config.WebhookProperties;
import com.mili.core.webhook.internal.infrastructure.messaging.InboundEventPublisher;
import com.mili.core.webhook.internal.infrastructure.persistence.WebhookConfigRepository;
import com.mili.core.webhook.internal.infrastructure.persistence.WebhookInboundConfigRepository;
import com.mili.core.webhook.internal.infrastructure.security.InboundAuthenticator;
import org.junit.jupiter.api.Test;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import org.assertj.core.api.SoftAssertions;

import java.lang.reflect.Constructor;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.reset;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

/** Independent no-database acceptance checks for MLI-03. */
class Mli03AcceptanceTest {

    @Test
    void cacheHitsArePreservedAndSuccessfulPauseAndDeleteInvalidateImmediately() throws Exception {
        var tokenRepository = mock(WebhookInboundConfigRepository.class);
        var configRepository = mock(WebhookConfigRepository.class);
        var tenantService = mock(TenantService.class);
        var authenticator = mock(InboundAuthenticator.class);
        var publisher = mock(InboundEventPublisher.class);
        var tenantId = new TenantId(UUID.randomUUID());
        var pauseId = UUID.randomUUID();
        var deleteId = UUID.randomUUID();
        var pauseToken = new IngestToken("pause-token-012345678901234567890123");
        var deleteToken = new IngestToken("delete-token-01234567890123456789012");
        var pauseConfig = config(pauseId, tenantId, "ACTIVE");
        var deleteConfig = config(deleteId, tenantId, "ACTIVE");
        Map<String, CachedWebhookConfig> authRows = new ConcurrentHashMap<>();
        authRows.put(pauseToken.value(), pauseConfig);
        authRows.put(deleteToken.value(), deleteConfig);
        Map<UUID, WebhookSummary> managementRows = new ConcurrentHashMap<>();
        managementRows.put(pauseId, summary(pauseId, "ACTIVE"));
        managementRows.put(deleteId, summary(deleteId, "ACTIVE"));

        when(tokenRepository.findAuthConfigByToken(any())).thenAnswer(invocation -> {
            IngestToken requested = invocation.getArgument(0);
            return Optional.ofNullable(authRows.get(requested.value()));
        });
        when(configRepository.findById(any())).thenAnswer(invocation ->
                Optional.ofNullable(managementRows.get(invocation.getArgument(0))));
        org.mockito.Mockito.doAnswer(invocation -> {
            UUID id = invocation.getArgument(0);
            String newStatus = invocation.getArgument(1);
            WebhookSummary old = managementRows.get(id);
            if (old != null) {
                managementRows.put(id, summary(id, newStatus));
                IngestToken rowToken = id.equals(pauseId) ? pauseToken : deleteToken;
                CachedWebhookConfig oldConfig = authRows.get(rowToken.value());
                if (oldConfig != null) authRows.put(rowToken.value(), config(id, tenantId, newStatus));
            }
            return null;
        }).when(configRepository).updateStatus(any(), any());
        org.mockito.Mockito.doAnswer(invocation -> {
            UUID id = invocation.getArgument(0);
            managementRows.remove(id);
            authRows.remove(id.equals(pauseId) ? pauseToken.value() : deleteToken.value());
            return null;
        }).when(configRepository).deleteById(any());

        var cache = new WebhookInboundCache(tokenRepository);
        WebhookInboundService managementService = createManagementService(
                tenantService, configRepository, tokenRepository, cache);
        Object ingestController = createIngestController(cache, authenticator, publisher);
        MockMvc mvc = org.springframework.test.web.servlet.setup.MockMvcBuilders
                .standaloneSetup(ingestController).build();
        when(authenticator.authenticate(any(), any(), any())).thenReturn(AuthResult.Allowed.INSTANCE);

        var assertions = new SoftAssertions();
        assertions.assertThat(cache.findByToken(pauseToken)).contains(pauseConfig);
        assertions.assertThat(cache.findByToken(pauseToken)).contains(pauseConfig);
        assertions.assertThat(repositoryReads(tokenRepository, pauseToken)).isEqualTo(1);

        managementService.updateStatus(pauseId, "PAUSED");
        assertions.assertThat(cache.findByToken(pauseToken)).contains(config(pauseId, tenantId, "PAUSED"));
        assertions.assertThat(repositoryReads(tokenRepository, pauseToken)).isEqualTo(2);
        reset(publisher);
        when(authenticator.authenticate(any(CachedWebhookConfig.class), any(java.net.InetAddress.class), any()))
                .thenReturn(AuthResult.Allowed.INSTANCE);
        MvcResult pausedRequest = mvc.perform(post("/ingest/{token}", pauseToken.value())
                        .content("{\"type\":\"test\"}"))
                .andReturn();
        assertions.assertThat(pausedRequest.getResponse().getStatus()).isEqualTo(404);
        assertions.assertThat(org.mockito.Mockito.mockingDetails(publisher).getInvocations()).isEmpty();
        assertions.assertThat(org.mockito.Mockito.mockingDetails(authenticator).getInvocations()).isEmpty();

        assertions.assertThat(cache.findByToken(deleteToken)).contains(deleteConfig);
        managementService.delete(deleteId);
        assertions.assertThat(cache.findByToken(deleteToken)).isEmpty();
        assertions.assertThat(repositoryReads(tokenRepository, deleteToken)).isEqualTo(2);
        reset(publisher, authenticator);
        when(authenticator.authenticate(any(CachedWebhookConfig.class), any(java.net.InetAddress.class), any()))
                .thenReturn(AuthResult.Allowed.INSTANCE);
        MvcResult deletedRequest = mvc.perform(post("/ingest/{token}", deleteToken.value())
                        .content("{\"type\":\"test\"}"))
                .andReturn();
        assertions.assertThat(deletedRequest.getResponse().getStatus()).isEqualTo(404);
        assertions.assertThat(org.mockito.Mockito.mockingDetails(publisher).getInvocations()).isEmpty();
        assertions.assertThat(org.mockito.Mockito.mockingDetails(authenticator).getInvocations()).isEmpty();
        assertions.assertAll();
    }

    private static CachedWebhookConfig config(UUID id, TenantId tenantId, String status) {
        return new CachedWebhookConfig(new WebhookId(id), tenantId, status,
                new IpAllowlist(List.of()), null, 10);
    }

    private static WebhookSummary summary(UUID id, String status) {
        return new WebhookSummary(id, "pilot", status, "http://localhost/ingest/pilot", Instant.EPOCH);
    }

    private static long repositoryReads(WebhookInboundConfigRepository repository, IngestToken token) {
        return org.mockito.Mockito.mockingDetails(repository).getInvocations().stream()
                .filter(invocation -> invocation.getMethod().getName().equals("findAuthConfigByToken"))
                .filter(invocation -> token.equals(invocation.getArgument(0)))
                .count();
    }

    private static WebhookInboundService createManagementService(
            TenantService tenants,
            WebhookConfigRepository configs,
            WebhookInboundConfigRepository inboundConfigs,
            WebhookInboundCache cache
    ) throws Exception {
        Class<?> implementation = Class.forName(
                "com.mili.core.webhook.internal.application.inbound.CreateInboundWebhookService");
        Constructor<?> constructor = java.util.Arrays.stream(implementation.getDeclaredConstructors())
                .filter(candidate -> java.util.Arrays.stream(candidate.getParameterTypes())
                        .anyMatch(type -> type == TenantService.class)
                        && java.util.Arrays.stream(candidate.getParameterTypes())
                        .anyMatch(type -> type == WebhookConfigRepository.class))
                .findFirst().orElseThrow();
        constructor.setAccessible(true);
        Object[] args = java.util.Arrays.stream(constructor.getParameterTypes())
                .map(type -> {
                    if (type == TenantService.class) return tenants;
                    if (type == WebhookConfigRepository.class) return configs;
                    if (type == WebhookInboundConfigRepository.class) return inboundConfigs;
                    if (type == WebhookProperties.class) return new WebhookProperties("http://localhost");
                    if (type == WebhookInboundCache.class) return cache;
                    throw new IllegalStateException("Unsupported service constructor dependency: " + type);
                }).toArray();
        return (WebhookInboundService) constructor.newInstance(args);
    }

    private static Object createIngestController(
            WebhookInboundCache cache,
            InboundAuthenticator authenticator,
            InboundEventPublisher publisher
    ) throws Exception {
        Class<?> implementation = Class.forName(
                "com.mili.core.webhook.internal.infrastructure.http.InboundController");
        Constructor<?> constructor = implementation.getDeclaredConstructor(
                WebhookInboundCache.class, InboundAuthenticator.class, InboundEventPublisher.class);
        constructor.setAccessible(true);
        return constructor.newInstance(cache, authenticator, publisher);
    }
}
