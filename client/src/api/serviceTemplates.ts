import { v1, wrap, type RequestOptions } from './http';

export interface TemplateCheck {
  type: string;
  name: string;
  enabled: boolean;
  interval: number;
}

export interface TemplateAlert {
  name: string;
  severity: string;
  metricField: string;
  operator: string;
  value: string;
}

export interface ServiceTemplate {
  name: string;
  description: string;
  category: string;
  deviceProtocol: string;
  devicePort: number;
  deviceCategory: string;
  checks: TemplateCheck[];
  alerts: TemplateAlert[];
}

export interface ApplyTemplateResult {
  deviceId: number;
  deviceName: string;
  sensorIds: number[];
  ruleIds: number[];
}

type RawTemplate = Omit<ServiceTemplate, 'checks' | 'alerts'> & {
  checks?: TemplateCheck[] | null;
  alerts?: TemplateAlert[] | null;
};

/** Normalises null check/alert lists from the backend to empty arrays. */
export const getServiceTemplates = ({ signal }: RequestOptions = {}) =>
  v1.get('/service-templates', { signal }).then((r) => {
    const data = wrap<RawTemplate[] | null>(r.data).data;
    const templates: ServiceTemplate[] = Array.isArray(data)
      ? data.map((template) => ({
          ...template,
          checks: Array.isArray(template.checks) ? template.checks : [],
          alerts: Array.isArray(template.alerts) ? template.alerts : [],
        }))
      : [];
    return { data: templates, success: true };
  });

export const applyServiceTemplate = (body: Record<string, string | number | boolean>, { signal }: RequestOptions = {}) =>
  v1.post('/service-templates/apply', body, { signal }).then((r) => wrap<ApplyTemplateResult>(r.data));
