/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package model

// offeredCurrencies are the currencies a paid plan can be priced in: the same list both portals
// offer (the API Portal's utils/currencies.js and the Management Portal's currencies.ts), and the
// API Portal refuses any other. It leaves out the currencies Stripe does not count in hundredths
// (its zero-decimal and three-decimal ones, and ISK, which it cannot charge in fractions): Moesif
// sends Stripe every amount multiplied by 100, so they would be charged 100 times or a tenth of
// the price. Change it together with the portals' lists.
var offeredCurrencies = map[string]bool{
	"AED": true, "AFN": true, "ALL": true, "AMD": true, "ANG": true, "AOA": true, "ARS": true, "AUD": true, "AWG": true, "AZN": true, "BAM": true, "BBD": true, "BDT": true, "BGN": true,
	"BMD": true, "BND": true, "BOB": true, "BRL": true, "BSD": true, "BWP": true, "BYN": true, "BZD": true, "CAD": true, "CDF": true, "CHF": true, "CNY": true, "COP": true, "CRC": true,
	"CVE": true, "CZK": true, "DKK": true, "DOP": true, "DZD": true, "EGP": true, "ETB": true, "EUR": true, "FJD": true, "FKP": true, "GBP": true, "GEL": true, "GIP": true, "GMD": true,
	"GTQ": true, "GYD": true, "HKD": true, "HNL": true, "HTG": true, "HUF": true, "IDR": true, "ILS": true, "INR": true, "JMD": true, "KES": true, "KGS": true, "KHR": true, "KYD": true,
	"KZT": true, "LAK": true, "LBP": true, "LKR": true, "LRD": true, "LSL": true, "MAD": true, "MDL": true, "MKD": true, "MMK": true, "MNT": true, "MOP": true, "MUR": true, "MVR": true,
	"MWK": true, "MXN": true, "MYR": true, "MZN": true, "NAD": true, "NGN": true, "NIO": true, "NOK": true, "NPR": true, "NZD": true, "PAB": true, "PEN": true, "PGK": true, "PHP": true,
	"PKR": true, "PLN": true, "QAR": true, "RON": true, "RSD": true, "RUB": true, "SAR": true, "SBD": true, "SCR": true, "SEK": true, "SGD": true, "SHP": true, "SOS": true, "SRD": true,
	"STD": true, "SZL": true, "THB": true, "TJS": true, "TOP": true, "TRY": true, "TTD": true, "TWD": true, "TZS": true, "UAH": true, "USD": true, "UYU": true, "UZS": true, "WST": true,
	"XCD": true, "YER": true, "ZAR": true, "ZMW": true,
}
