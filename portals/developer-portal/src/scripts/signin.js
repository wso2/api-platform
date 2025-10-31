/*
 * Copyright (c) 2025, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 */
document.addEventListener('DOMContentLoaded', function () {
    const signinCard = document.getElementById('signin-card');
    const enterpriseCard = document.getElementById('enterprise-card');
    const baseUrl = enterpriseCard.getAttribute('data-base-url');

    const showEnterpriseBtn = document.getElementById('show-enterprise');
    const backBtn = document.getElementById('back-to-signin');

    showEnterpriseBtn.addEventListener('click', function () {
        signinCard.style.display = 'none';
        enterpriseCard.style.display = 'block';
    });

    backBtn.addEventListener('click', function () {
        enterpriseCard.style.display = 'none';
        signinCard.style.display = 'block';
    });

    document.getElementById('login-google').addEventListener('click', function () {
        window.location.href = `${baseUrl}/login?fidp=google`;
    });

    document.getElementById('login-github').addEventListener('click', function () {
        window.location.href = `${baseUrl}/login?fidp=github`;
    });

    document.getElementById('login-microsoft').addEventListener('click', function () {
        window.location.href = `${baseUrl}/login?fidp=microsoft`;
    });

    document.getElementById('login-email').addEventListener('click', function () {
        window.location.href = `${baseUrl}/login?fidp=email`;
    });

    const emailInput = document.getElementById('enterprise-email');
    const continueBtn = document.getElementById('login-enterprise');

    continueBtn.addEventListener('click', function (e) {
        const email = emailInput.value.trim();
        if (!validateEmail(email)) {
            alert("Please enter a valid email address.");
            return;
        }

        const redirectUrl = `${baseUrl}/login?fidp=enterprise&username=${encodeURIComponent(email)}`;
        window.location.href = redirectUrl;
    });

    emailInput.addEventListener('input', () => {
        const email = emailInput.value.trim();
        const isValid = validateEmail(email);

        continueBtn.disabled = !isValid;
        continueBtn.style.opacity = isValid ? '1' : '0.5';
    });

    function validateEmail(email) {
        return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
    }
});
