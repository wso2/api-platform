import type { ApiPublishPayload } from '../../../hooks/apiPublish';

type FormLike = {
  apiName: string;
  productionURL: string;
  sandboxURL: string;
  apiDescription?: string;
  visibility?: 'PUBLIC' | 'PRIVATE';
  tags?: string[];
  labels?: string[];
  technicalOwner?: string;
  technicalOwnerEmail?: string;
  businessOwner?: string;
  businessOwnerEmail?: string;
  subscriptionPolicies?: string[];
};

export const buildPublishPayload = (
  form: FormLike,
  devPortalUUID: string
): ApiPublishPayload => {
  return {
    devPortalUUID,
    endPoints: {
      productionURL: form.productionURL,
      sandboxURL: form.sandboxURL,
    },
    apiInfo: {
      apiName: form.apiName,
      apiDescription: form.apiDescription ?? '',
      visibility: form.visibility ?? 'PUBLIC',
      tags: form.tags ?? [],
      labels: ['default'],
      owners: {
        technicalOwner: form.technicalOwner ?? '',
        technicalOwnerEmail: form.technicalOwnerEmail ?? '',
        businessOwner: form.businessOwner ?? '',
        businessOwnerEmail: form.businessOwnerEmail ?? '',
      },
    },
    subscriptionPolicies: ['Default'],
  };
};

export default buildPublishPayload;
