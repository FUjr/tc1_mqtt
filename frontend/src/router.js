import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    name: 'Console',
    component: () => import('./App.vue')
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('./views/RegisterPage.vue')
  },
  {
    path: '/admin',
    name: 'Admin',
    component: () => import('./views/AdminPage.vue')
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router
