package com.mili.core.webhook.pilot;

import com.mili.core.webhook.WebhookMonitoringService;
import com.mili.core.webhook.api.InboundEventDetail;
import com.mili.core.webhook.internal.infrastructure.persistence.InboundEventRepository;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.test.web.servlet.MockMvc;

import java.lang.reflect.Constructor;
import java.time.Instant;
import java.util.Optional;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.*;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;
import static org.springframework.test.web.servlet.setup.MockMvcBuilders.standaloneSetup;

/**
 * Independent MLI-04 acceptance checks. This source is frozen outside the
 * product repository and exercises the actual service, controller, and MVC
 * exception advice with only the persistence repository mocked.
 */
class Mli04AcceptanceTest {

    private static final String MISSING_ID = "01ARZ3NDEKTSV4RRFFQ69G5FAV";

    private InboundEventRepository repository;
    private WebhookMonitoringService service;
    private MockMvc mvc;

    @BeforeEach
    void setUp() throws Exception {
        repository = mock(InboundEventRepository.class);
        service = construct(
                "com.mili.core.webhook.internal.application.monitoring.WebhookMonitoringServiceImpl",
                new Class<?>[]{InboundEventRepository.class}, repository);
        Object controller = construct(
                "com.mili.core.webhook.internal.infrastructure.http.WebhookMonitoringController",
                new Class<?>[]{WebhookMonitoringService.class}, service);
        Object advice = construct(
                "com.mili.core.webhook.internal.infrastructure.http.GlobalExceptionHandler",
                new Class<?>[0]);
        mvc = standaloneSetup(controller).setControllerAdvice(advice).build();
    }

    @Test
    void missingEventIsMappedToNotFoundByRealMvcAdvice() throws Exception {
        when(repository.findDetailById(MISSING_ID)).thenReturn(Optional.empty());

        var response = mvc.perform(post("/v1/inbound-events/{id}/reprocess", MISSING_ID))
                .andExpect(status().isNotFound())
                .andReturn().getResponse();

        assertThat(response.getContentType()).contains("application/problem+json");
        assertThat(response.getContentAsString())
                .doesNotContain("IllegalArgumentException")
                .doesNotContain("Event not found: " + MISSING_ID);
        verify(repository).findDetailById(MISSING_ID);
        verify(repository, never()).resetToPending(anyString());
    }

    @Test
    void serviceUsesNotFoundExceptionForAbsentEvent() {
        when(repository.findDetailById(MISSING_ID)).thenReturn(Optional.empty());

        assertThatThrownBy(() -> service.reprocess(MISSING_ID))
                .isNotInstanceOf(IllegalArgumentException.class);

        verify(repository).findDetailById(MISSING_ID);
        verify(repository, never()).resetToPending(anyString());
    }

    @Test
    void failedEventReturnsNoContentAndResetsExactlyOnce() throws Exception {
        String id = "01ARZ3NDEKTSV4RRFFQ69G5FAW";
        when(repository.findDetailById(id)).thenReturn(Optional.of(detail(id, "FAILED")));

        mvc.perform(post("/v1/inbound-events/{id}/reprocess", id))
                .andExpect(status().isNoContent());

        verify(repository).findDetailById(id);
        verify(repository, times(1)).resetToPending(id);
    }

    @Test
    void nonFailedEventReturnsConflictAndIsNotReset() throws Exception {
        String id = "01ARZ3NDEKTSV4RRFFQ69G5FAX";
        when(repository.findDetailById(id)).thenReturn(Optional.of(detail(id, "PROCESSED")));

        mvc.perform(post("/v1/inbound-events/{id}/reprocess", id))
                .andExpect(status().isConflict());

        verify(repository).findDetailById(id);
        verify(repository, never()).resetToPending(anyString());
    }

    private static InboundEventDetail detail(String id, String status) {
        return new InboundEventDetail(id, UUID.randomUUID(), UUID.randomUUID(),
                "order.created", "{}", "{}", status,
                "FAILED".equals(status) ? "timeout" : null,
                Instant.parse("2025-01-01T00:00:00Z"),
                "FAILED".equals(status) ? null : Instant.parse("2025-01-01T00:01:00Z"));
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
