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
