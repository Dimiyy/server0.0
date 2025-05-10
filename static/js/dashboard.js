document.addEventListener('DOMContentLoaded', function() {
    const tabs = document.querySelectorAll('.tab');
    
    tabs.forEach(tab => {
        tab.addEventListener('click', function() {
            // Удаляем активный класс у всех вкладок и контента
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
            
            // Добавляем активный класс текущей вкладке и соответствующему контенту
            this.classList.add('active');
            const tabId = this.getAttribute('data-tab');
            document.getElementById(tabId).classList.add('active');
        });
    });
    
    // Обработчик кнопки выхода
    document.querySelector('.logout-btn').addEventListener('click', function() {
        if (confirm('Вы уверены, что хотите выйти из системы?')) {
            window.location.href = '/logout'; // Нужно будет добавить обработчик logout в Go
        }
    });
});