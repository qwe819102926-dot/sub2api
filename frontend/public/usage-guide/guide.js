(() => {
  const lightbox = document.getElementById('lightbox')
  const lightboxImage = document.getElementById('lightbox-image')

  if (lightbox && lightboxImage) {
    const closeLightbox = () => {
      lightbox.classList.remove('open')
      lightboxImage.src = ''
      document.body.style.overflow = ''
    }

    document.querySelectorAll('[data-lightbox]').forEach((trigger) => {
      trigger.addEventListener('click', () => {
        lightboxImage.src = trigger.dataset.lightbox
        lightboxImage.alt = trigger.querySelector('img')?.alt || '教程截图'
        lightbox.classList.add('open')
        document.body.style.overflow = 'hidden'
      })
    })

    lightbox.addEventListener('click', (event) => {
      if (event.target === lightbox) closeLightbox()
    })
    lightbox.querySelector('.lightbox-close')?.addEventListener('click', closeLightbox)
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape' && lightbox.classList.contains('open')) closeLightbox()
    })
  }

  const copyText = async (value) => {
    if (navigator.clipboard && window.isSecureContext) {
      try {
        await navigator.clipboard.writeText(value)
        return true
      } catch {
        // Fall through to the HTTP-compatible selection fallback.
      }
    }

    const textarea = document.createElement('textarea')
    textarea.value = value
    textarea.setAttribute('readonly', '')
    textarea.style.position = 'fixed'
    textarea.style.top = '0'
    textarea.style.left = '-9999px'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.focus()
    textarea.select()
    textarea.setSelectionRange(0, textarea.value.length)
    let copied = false
    try {
      copied = document.execCommand('copy')
    } catch {
      copied = false
    }
    textarea.remove()
    return copied
  }

  document.querySelectorAll('[data-copy]').forEach((button) => {
    button.addEventListener('click', async () => {
      const value = button.dataset.copy || ''
      const copied = await copyText(value)
      button.textContent = copied ? '已复制' : '请手动复制'
      window.setTimeout(() => { button.textContent = '复制' }, 1800)
    })
  })

  document.querySelectorAll('[data-complete]').forEach((button) => {
    button.addEventListener('click', () => {
      const step = document.getElementById(button.dataset.complete)
      if (!step) return
      const completed = step.classList.toggle('is-done')
      button.textContent = completed ? '✓ 已完成' : '○ 标记已完成'
      button.setAttribute('aria-pressed', String(completed))
    })
  })

  document.getElementById('print-button')?.addEventListener('click', () => window.print())

  const navLinks = [...document.querySelectorAll('.side-nav a')]
  const sections = navLinks.map((link) => document.querySelector(link.getAttribute('href')))
  const updateActiveNav = () => {
    const marker = window.scrollY + 150
    let activeIndex = 0
    sections.forEach((section, index) => {
      if (section && section.offsetTop <= marker) activeIndex = index
    })
    navLinks.forEach((link, index) => link.classList.toggle('active', index === activeIndex))
  }
  window.addEventListener('scroll', updateActiveNav, { passive: true })
  updateActiveNav()
})()
