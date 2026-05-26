/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

/*
 * Tag: Invoices
 */
const invoiceController = require('../../controllers/invoiceController');
const { ensureBillingAuth } = require('../../middlewares/billingAuth');
const { compose } = require('./_compose');

module.exports = {
    listInvoices: compose(ensureBillingAuth, invoiceController.listInvoices),
    getInvoice: compose(ensureBillingAuth, invoiceController.getInvoice),
    listInvoicesBySubscription: compose(ensureBillingAuth, invoiceController.listInvoicesBySubscription),
    getInvoicePdfLink: compose(ensureBillingAuth, invoiceController.getInvoicePdfLink),
    redirectHostedInvoice: compose(ensureBillingAuth, invoiceController.redirectHostedInvoice),
};
