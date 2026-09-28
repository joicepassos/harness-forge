package com.mili.core.webhook.pilot;

import com.mili.core.webhook.WebhookMonitoringService;
import com.mili.core.webhook.api.InboundEventSummary;
import com.mili.core.webhook.internal.infrastructure.persistence.InboundEventRepository;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.test.web.servlet.MockMvc;

import java.lang.reflect.Constructor;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;
import static org.springframework.test.web.servlet.setup.MockMvcBuilders.standaloneSetup;

/** Independent parser/controller acceptance checks for MLI-02. */
class Mli02AcceptanceTest {
    private InboundEventRepository repository;
    private WebhookMonitoringService service;
    private MockMvc mvc;
    private UUID tenant;

    @BeforeEach
    void setUp() throws Exception {
        repository = mock(InboundEventRepository.class);
        service = construct(
                "com.mili.core.webhook.internal.application.monitoring.WebhookMonitoringServiceImpl",
                new Class<?>[]{InboundEventRepository.class}, repository);
        Object controller = construct(
                "com.mili.core.webhook.internal.infrastructure.http.WebhookMonitoringController",
                new Class<?>[]{WebhookMonitoringService.class}, service);
        mvc = standaloneSetup(controller).build();
        tenant = UUID.randomUUID();
    }

    @Test
    void blankCursorStartsFirstPageWithNoBoundary() throws Exception {
        mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString()))
                .andExpect(status().isOk());
        mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString()).param("pageAfter", " "))
                .andExpect(status().isOk());

        verify(repository, times(2)).findSummaries(eq(tenant), isNull(), isNull(), isNull(), isNull());
    }

    @Test
    void parsesFixedCursorAndRejectsMalformedCursorWithoutRepositoryQuery() throws Exception {
        Instant expected = Instant.ofEpochMilli(1730000000123L);
        mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString())
                        .param("pageAfter", "1730000000123|evt-2"))
                .andExpect(status().isOk());
        verify(repository).findSummaries(eq(tenant), isNull(), isNull(), eq(expected), eq("evt-2"));

        clearInvocations(repository);
        for (String cursor : List.of("abc|evt-2", "1730000000123", "1730000000123|",
                "9223372036854775808|evt-2")) {
            mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString())
                            .param("pageAfter", cursor))
                    .andExpect(status().isBadRequest());
        }
        verifyNoInteractions(repository);
    }

    @Test
    void emitsCursorFromLastItemAndUsesItAsTheNextPageBoundary() throws Exception {
        Instant receivedAt = Instant.parse("2026-09-27T12:00:00Z");
        List<InboundEventSummary> firstPage = new ArrayList<>();
        for (int i = 0; i < 50; i++) {
            firstPage.add(new InboundEventSummary("evt-" + i, tenant, UUID.randomUUID(), "type",
                    "PROCESSED", receivedAt.minusSeconds(i)));
        }
        when(repository.findSummaries(eq(tenant), isNull(), isNull(), isNull(), isNull()))
                .thenReturn(firstPage);
        String cursor = receivedAt.minusSeconds(49).toEpochMilli() + "|evt-49";
        mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString()))
                .andExpect(status().isOk())
                .andReturn().getResponse().getContentAsString();
        var response = mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString()))
                .andExpect(status().isOk()).andReturn().getResponse().getContentAsString();
        assertThat(response).contains(cursor);

        mvc.perform(get("/v1/inbound-events").param("tenantId", tenant.toString()).param("pageAfter", cursor))
                .andExpect(status().isOk());
        verify(repository).findSummaries(eq(tenant), isNull(), isNull(),
                eq(receivedAt.minusSeconds(49)), eq("evt-49"));
    }

    @SuppressWarnings("unchecked")
    private static <T> T construct(String className, Class<?>[] parameterTypes, Object... args)
            throws Exception {
        Class<?> type = Class.forName(className);
        Constructor<?> constructor = type.getDeclaredConstructor(parameterTypes);
        constructor.setAccessible(true);
        return (T) constructor.newInstance(args);
    }
}
