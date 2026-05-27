// ***** Dashboard Naivgation *****

document.addEventListener('DOMContentLoaded', function () {
  const navLinks = document.querySelectorAll('.nav-link[data-section]');
  const sections = document.querySelectorAll('.content-section');

  navLinks.forEach((link) => {
    link.addEventListener('click', (event) => {
      event.preventDefault();
      const targetSectionId = link.getAttribute('data-section');

      // Hide all sections
      sections.forEach((section) => section.classList.add('d-none'));

      // Show the target section
      const targetSection = document.getElementById(targetSectionId);
      if (targetSection) {
        targetSection.classList.remove('d-none');
      }
    });
  });
});

// ***** Throttling Policy Reset Modal *****

// Open the Throttling Policy Reset Modal

function openResetModal() {
  const modal = document.getElementById('throttlingPolicyResetModal');
  const bootstrapModal = new bootstrap.Modal(modal);
  bootstrapModal.show();
}

// Validation of the Reset Throttling Policy Form

document.addEventListener('DOMContentLoaded', () => {
  const userName = document.getElementById('userName');
  const userNameError = document.getElementById('userNameError');
  const resetButton = document.getElementById('resetButton');

  if (!userName || !userNameError || !resetButton) return;

  const validateForm = () => {
    let hasError = false;
    if (!userName.value.trim()) {
      userNameError.style.display = 'block';
      hasError = true;
    } else {
      userNameError.style.display = 'none';
    }

    resetButton.disabled = hasError;
  };
  userName.addEventListener('input', validateForm);
});

// Reset Throttling Policy Submit

async function resetThrottlingPolicy(applicationId) {
  const userName = document.getElementById('userName').value;

  try {
    const response = await fetch(
      `/devportal/applications/${applicationId}/reset-throttle-policy`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          userName,
        }),
      }
    );
    if (response.ok) {
      await showAlert('Throttling policy reset successfully!', 'success');
    } else {
      await showAlert(
        'Failed to reset throttling policy. Please try again.',
        'error'
      );
    }
  } catch (error) {
    await showAlert(
      'An error occurred while resetting the throttling policy. Please try again.',
      'error'
    );
  }
}
